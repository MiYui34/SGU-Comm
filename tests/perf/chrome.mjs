// A headless Chromium with a fresh profile, driven over the raw DevTools
// protocol (no npm packages). CHROME: the binary (see README.md).
import { spawn } from "node:child_process";
import { once } from "node:events";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// init: a script that runs in the page before the app; cpu: a slowdown.
export const launch = async ({ init, cpu = 1 } = {}) => {
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
  const waitFor = async (expr, timeout = 20000) => {
    for (const end = Date.now() + timeout; Date.now() < end; await sleep(20)) if (await evaluate(expr)) return true;
    throw new Error("timeout: " + expr);
  };
  await cdp("Page.enable");
  await cdp("Runtime.enable");
  if (init) await cdp("Page.addScriptToEvaluateOnNewDocument", { source: init });
  if (cpu > 1) await cdp("Emulation.setCPUThrottlingRate", { rate: cpu });
  const close = async () => {
    ws.close();
    chrome.kill();
    await once(chrome, "exit");
    rmSync(profile, { recursive: true, force: true, maxRetries: 5 });
  };
  return { cdp, evaluate, waitFor, close };
};

// Logs in as a guest with the name (the app's login page).
export const loginAsGuest = async ({ evaluate, waitFor }, name) => {
  await waitFor(`!!document.querySelector("input[required]")`);
  await evaluate(
    `(() => { const i = document.querySelector("input[required]"); i.value = ${JSON.stringify(name)}; i.dispatchEvent(new Event("input", { bubbles: true })); document.querySelector("button[type=submit]").click(); })()`,
  );
};

// The General voice channel's button in the sidebar.
export const GENERAL = `[...document.querySelectorAll("nav button")].find((b) => /^\\s*General/.test(b.textContent))`;
