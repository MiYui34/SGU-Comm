// Opens the app in headless Chromium (raw CDP, no dependencies), joins as a
// guest, joins the General voice channel at once, and saves per message on
// the socket its type, bytes and arrival, plus the playback counters every
// 100 ms (see apps/web/src/lib/perfHook.ts) and long tasks.
//
//   CHROME=<headless chromium> node measure.mjs <app URL> <out.json> [cpu slowdown]
//
// DURATION: seconds after the join (default 15).
import { writeFileSync } from "node:fs";
import { GENERAL, launch, loginAsGuest, sleep } from "./chrome.mjs";

const [url, out, cpuArg] = process.argv.slice(2);
if (!url || !out) throw new Error("usage: node measure.mjs <app URL> <out.json> [cpu slowdown]");
const cpu = Number(cpuArg ?? 1);
const DURATION = Number(process.env.DURATION ?? 15);

// Runs in the page before the app: a silent microphone, the perf hook, and
// a WebSocket wrapper that splits the Mumble socket's stream into frames.
const instrument = `(() => {
  const log = (window.__warm = { frames: [], stats: [], long: [], sent: [], t0: null });
  const perf = (window.__kallistiPerf = {});
  const now = () => performance.now();
  new PerformanceObserver((l) => l.getEntries().forEach((e) => log.long.push([e.startTime, e.duration]))).observe({ type: "longtask", buffered: true });
  const md = navigator.mediaDevices;
  md.enumerateDevices = async () => [];
  md.getUserMedia = async () => {
    const c = new AudioContext({ sampleRate: 48000 });
    const o = c.createOscillator(); const g = c.createGain(); g.gain.value = 0;
    const d = c.createMediaStreamDestination(); o.connect(g); g.connect(d); o.start();
    return d.stream;
  };
  const Orig = window.WebSocket;
  window.WebSocket = class extends Orig {
    constructor(...a) {
      super(...a);
      if (String(a[0]).includes("/api/stream")) return;
      log.t0 ??= now();
      let buf = new Uint8Array(0);
      this.addEventListener("message", (e) => {
        const t = now();
        if (typeof e.data === "string") {
          let type = "?"; try { type = JSON.parse(e.data).type; } catch {}
          log.frames.push([t, "json:" + type, e.data.length]);
          return;
        }
        const c = new Uint8Array(e.data);
        const b = new Uint8Array(buf.length + c.length); b.set(buf); b.set(c, buf.length); buf = b;
        while (buf.length >= 6) {
          const len = new DataView(buf.buffer, buf.byteOffset).getUint32(2);
          if (buf.length < 6 + len) break;
          log.frames.push([t, "mumble:" + ((buf[0] << 8) | buf[1]), len + 6]);
          buf = buf.slice(6 + len);
        }
      });
      const send = this.send.bind(this);
      this.send = (d) => { log.sent.push([now(), typeof d === "string" ? d.slice(0, 60) : "bin:" + d.byteLength]); send(d); };
      setInterval(() => { const s = perf.playback?.(); if (s) log.stats.push([now(), s.good, s.late, s.lost]); }, 100);
    }
  };
})();`;

const page = await launch({ init: instrument, cpu });
const { cdp, evaluate, waitFor } = page;
await cdp("Page.navigate", { url });
await loginAsGuest(page, `listener${Math.floor(Math.random() * 1e4)}`);
await waitFor(`!!${GENERAL}`);
await evaluate(`${GENERAL}.click(); window.__warm.joined = performance.now();`);
await sleep(DURATION * 1000);
const avatars = await evaluate(`[...document.querySelectorAll("img")].filter((i) => i.naturalWidth && /avatars|data:image/.test(i.src)).length`);
writeFileSync(out, await evaluate(`JSON.stringify(window.__warm)`));
await page.close();
console.log(`saved ${out} (${avatars} avatars shown)`);
