// A screen share and its viewers, each in its own headless Chromium (raw
// CDP): the streamer shares a fake "tab" (a canvas at 30 fps, high motion
// like a video, with sound unless SOUND=0); each viewer joins General and
// watches it. Saves per page what went over the stream socket and when
// pictures were drawn and sound was scheduled (see stream-analyze.mjs).
//
//   CHROME=<headless chromium> node stream.mjs <out.json> <streamer URL> <viewer URL>...
//
// DURATION: seconds of watching (default 20). SOUND: 0 for a silent share.
// MOTION: "video" (default; noise, gradients, moving shapes) or "tab"
// (scrolling text: little change per frame).
import { writeFileSync } from "node:fs";
import { GENERAL, launch, loginAsGuest, sleep } from "./chrome.mjs";

const [out, streamerUrl, ...viewerUrls] = process.argv.slice(2);
if (!out || !streamerUrl || !viewerUrls.length) throw new Error("usage: node stream.mjs <out.json> <streamer URL> <viewer URL>...");
const DURATION = Number(process.env.DURATION ?? 20);
const SOUND = process.env.SOUND !== "0";
const MOTION = process.env.MOTION ?? "video";

// Both: wall-clock times (comparable across the browsers), a silent
// microphone, and the stream socket's frames: [time, flags, timestamp,
// bytes] (flags: 1 key, 2 sound).
const common = `
  const log = (window.__stream = { sent: [], got: [], texts: [], draws: [], sounds: [], encodes: [], captured: 0 });
  const now = () => performance.timeOrigin + performance.now();
  const md = navigator.mediaDevices;
  md.enumerateDevices = async () => [];
  md.getUserMedia = async () => {
    const c = new AudioContext({ sampleRate: 48000 });
    const o = c.createOscillator(); const g = c.createGain(); g.gain.value = 0;
    const d = c.createMediaStreamDestination(); o.connect(g); g.connect(d); o.start();
    return d.stream;
  };
  const header = (view, at) => [view.getUint8(at), Number(view.getBigUint64(at + 1))];
  const Orig = window.WebSocket;
  window.WebSocket = class extends Orig {
    constructor(...a) {
      super(...a);
      if (!String(a[0]).includes("/api/stream")) return;
      this.addEventListener("message", (e) => {
        if (typeof e.data === "string") return log.texts.push([now(), JSON.parse(e.data).type]);
        log.got.push([now(), ...header(new DataView(e.data), 4), e.data.byteLength - 13]);
      });
      const send = this.send.bind(this);
      this.send = (d) => {
        if (typeof d === "string") log.texts.push([now(), "> " + JSON.parse(d).type]);
        else log.sent.push([now(), ...header(new DataView(d.buffer, d.byteOffset), 0), d.byteLength - 9, this.bufferedAmount]);
        send(d);
      };
    }
  };`;

// The streamer: getDisplayMedia gives the canvas (and an oscillator); the
// app's frames into the encoder are counted.
const streamer = `(() => {${common}
  const W = 1280, H = 720, motion = ${JSON.stringify(MOTION)}, sound = ${SOUND};
  const canvas = document.createElement("canvas"); canvas.width = W; canvas.height = H;
  const ctx = canvas.getContext("2d");
  const noise = Array.from({ length: 4 }, () => {
    const c = new OffscreenCanvas(320, 180); const x = c.getContext("2d"); const img = x.createImageData(320, 180);
    for (let i = 0; i < img.data.length; i++) img.data[i] = i % 4 === 3 ? 255 : Math.random() * 256;
    x.putImageData(img, 0, 0); return c;
  });
  let n = 0;
  const paint = () => {
    n++;
    if (motion === "video") {
      const g = ctx.createLinearGradient(0, 0, W, H);
      g.addColorStop(0, "hsl(" + (n * 3) % 360 + ",70%,40%)"); g.addColorStop(1, "hsl(" + (n * 3 + 180) % 360 + ",70%,50%)");
      ctx.fillStyle = g; ctx.fillRect(0, 0, W, H);
      ctx.globalAlpha = 0.35; ctx.drawImage(noise[n % 4], -(n * 7) % 160, -(n * 5) % 90, W + 160, H + 90); ctx.globalAlpha = 1;
      for (let i = 0; i < 12; i++) {
        ctx.fillStyle = "hsl(" + i * 30 + ",80%,60%)"; ctx.beginPath();
        ctx.arc((W / 2) + Math.sin(n / 20 + i) * W * 0.4, (H / 2) + Math.cos(n / 15 + i * 2) * H * 0.4, 30 + i * 4, 0, 7); ctx.fill();
      }
    } else {
      ctx.fillStyle = "#fff"; ctx.fillRect(0, 0, W, H); ctx.fillStyle = "#222"; ctx.font = "18px sans-serif";
      for (let y = -(n * 2) % 24; y < H; y += 24) ctx.fillText("Line " + Math.floor((y + n * 2) / 24) + " lorem ipsum dolor sit amet, consectetur adipiscing elit", 40, y);
    }
    ctx.fillStyle = "#000"; ctx.fillRect(0, 0, 160, 40); ctx.fillStyle = "#fff"; ctx.font = "28px monospace"; ctx.fillText(String(n), 8, 30);
  };
  setInterval(paint, 1000 / 30);
  md.getDisplayMedia = async () => {
    const tracks = canvas.captureStream(30).getVideoTracks();
    if (sound) {
      const c = new AudioContext({ sampleRate: 48000 });
      const o = c.createOscillator(); o.frequency.value = 440;
      const d = c.createMediaStreamDestination(); o.connect(d); o.start();
      tracks.push(d.stream.getAudioTracks()[0]);
    }
    return new MediaStream(tracks);
  };
  const OrigFrame = window.VideoFrame;
  window.VideoFrame = class extends OrigFrame { constructor(...a) { super(...a); log.captured++; } };
  const encode = VideoEncoder.prototype.encode;
  VideoEncoder.prototype.encode = function (f, o) { log.encodes.push([now(), this.encodeQueueSize]); return encode.call(this, f, o); };
})();`;

// A viewer: drawn stream pictures [time, timestamp, audio clock] and
// scheduled sound [time, when, audio clock, timestamp] (the stream's sound
// is scheduled in its decoder's output, right after the data).
const viewer = `(() => {${common}
  let ctx = null, soundTs = null;
  const drawImage = CanvasRenderingContext2D.prototype.drawImage;
  CanvasRenderingContext2D.prototype.drawImage = function (img, ...a) {
    if (img instanceof VideoFrame) log.draws.push([now(), img.timestamp, ctx?.currentTime ?? null]);
    return drawImage.call(this, img, ...a);
  };
  const start = AudioBufferSourceNode.prototype.start;
  AudioBufferSourceNode.prototype.start = function (when = 0, ...a) {
    ctx = this.context;
    log.sounds.push([now(), when, ctx.currentTime, soundTs]);
    return start.call(this, when, ...a);
  };
  const Decoder = window.AudioDecoder;
  window.AudioDecoder = class extends Decoder {
    constructor(init) { super({ ...init, output: (d) => { soundTs = d.timestamp; init.output(d); } }); }
  };
})();`;

const join = async (url, init, name) => {
  const page = await launch({ init });
  await page.cdp("Page.navigate", { url });
  await loginAsGuest(page, name);
  await page.waitFor(`!!${GENERAL}`);
  await page.evaluate(`${GENERAL}.click()`);
  return page;
};

const name = `streamer${Math.floor(Math.random() * 1e4)}`;
const s = await join(streamerUrl, streamer, name);
await s.waitFor(`!!document.querySelector("button[aria-label='Go live']")`);
await s.evaluate(`document.querySelector("button[aria-label='Go live']").click()`);
await s.waitFor(`!![...document.querySelectorAll("button.item")].find((b) => b.textContent.includes("Share your screen"))`);
await s.evaluate(`[...document.querySelectorAll("button.item")].find((b) => b.textContent.includes("Share your screen")).click()`);
await s.waitFor(`window.__stream.encodes.length > 0`);
const viewers = [];
for (const [i, url] of viewerUrls.entries()) {
  const v = await join(url, viewer, `viewer${i}x${Math.floor(Math.random() * 1e4)}`);
  const watch = `document.querySelector("button[title^='Watch ${name}']")`;
  await v.waitFor(`!!${watch}`);
  await v.evaluate(`${watch}.click()`);
  viewers.push({ page: v, url, watched: Date.now() });
}
const watched = Date.now();
await sleep(DURATION * 1000);
const result = { duration: DURATION, sound: SOUND, motion: MOTION, watched, streamer: JSON.parse(await s.evaluate(`JSON.stringify(window.__stream)`)), viewers: [] };
for (const { page, url, watched } of viewers) result.viewers.push({ url, watched, ...JSON.parse(await page.evaluate(`JSON.stringify(window.__stream)`)) });
writeFileSync(out, JSON.stringify(result));
await Promise.all([s, ...viewers.map((v) => v.page)].map((p) => p.close()));
console.log(`saved ${out}`);
