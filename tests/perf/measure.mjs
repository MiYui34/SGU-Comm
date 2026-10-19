// Opens the app in headless Chromium (raw CDP, no dependencies), joins as a
// guest, joins the General voice channel at once, and saves per message on
// the socket its type, bytes and arrival, plus the playback counters every
// 100 ms (see apps/web/src/lib/perfHook.ts) and long tasks.
//
//   CHROME=<headless chromium> node measure.mjs <app URL> <out.json> [cpu slowdown]
//
// DURATION: seconds after the join (default 15).
import { spawn } from "node:child_process";
import { once } from "node:events";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const [url, out, cpuArg] = process.argv.slice(2);
if (!url || !out) throw new Error("usage: node measure.mjs <app URL> <out.json> [cpu slowdown]");
const cpu = Number(cpuArg ?? 1);
const DURATION = Number(process.env.DURATION ?? 15);
const CHROME = process.env.CHROME;
if (!CHROME) throw new Error("set CHROME to a Chromium binary (see README.md)");

const port = 9300 + Math.floor(Math.random() * 600);
const profile = mkdtempSync(join(tmpdir(), "kallisti-perf-chrome-"));
// --headless: also keeps a full Chrome off the screen (the headless shell
// ignores it).
const chrome = spawn(
  CHROME,
  [
    "--headless",
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profile}`,
    "--autoplay-policy=no-user-gesture-required",
    "--use-fake-ui-for-media-stream",
    "--use-fake-device-for-media-stream",
    "--no-first-run",
    "--no-sandbox",
    "--disable-gpu",
    "about:blank",
  ],
  { stdio: "ignore" },
);
process.on("exit", () => chrome.kill());
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

let target;
for (let i = 0; i < 50 && !target; i++) {
  await sleep(200);
  try {
    target = (await (await fetch(`http://127.0.0.1:${port}/json`)).json()).find((t) => t.type === "page");
  } catch {}
}
if (!target) throw new Error(`no DevTools on port ${port}: is CHROME a Chromium?`);
const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise((resolve, reject) => {
  ws.onopen = resolve;
  ws.onerror = () => reject(new Error("DevTools connection failed"));
});
let id = 0;
const waiting = new Map();
ws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.id && waiting.has(m.id)) waiting.get(m.id)(m), waiting.delete(m.id);
};
const cdp = (method, params = {}) =>
  new Promise((resolve) => {
    const n = ++id;
    waiting.set(n, resolve);
    ws.send(JSON.stringify({ id: n, method, params }));
  });
const evaluate = async (expr) => (await cdp("Runtime.evaluate", { expression: expr, returnByValue: true, awaitPromise: true })).result?.result?.value;

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

await cdp("Page.enable");
await cdp("Runtime.enable");
await cdp("Page.addScriptToEvaluateOnNewDocument", { source: instrument });
if (cpu > 1) await cdp("Emulation.setCPUThrottlingRate", { rate: cpu });
await cdp("Page.navigate", { url });
const waitFor = async (expr, timeout = 20000) => {
  for (const end = Date.now() + timeout; Date.now() < end; await sleep(20)) if (await evaluate(expr)) return true;
  throw new Error("timeout: " + expr);
};
const general = `[...document.querySelectorAll("nav button")].find((b) => /^\\s*General/.test(b.textContent))`;
await waitFor(`!!document.querySelector("input[required]")`);
await evaluate(`(() => { const i = document.querySelector("input[required]"); i.value = "listener${Math.floor(Math.random() * 1e4)}"; i.dispatchEvent(new Event("input", { bubbles: true })); document.querySelector("button[type=submit]").click(); })()`);
await waitFor(`!!${general}`);
await evaluate(`${general}.click(); window.__warm.joined = performance.now();`);
await sleep(DURATION * 1000);
const avatars = await evaluate(`[...document.querySelectorAll("img")].filter((i) => i.naturalWidth && /avatars|data:image/.test(i.src)).length`);
writeFileSync(out, await evaluate(`JSON.stringify(window.__warm)`));
ws.close();
chrome.kill();
await once(chrome, "exit");
rmSync(profile, { recursive: true, force: true, maxRetries: 5 });
console.log(`saved ${out} (${avatars} avatars shown)`);
