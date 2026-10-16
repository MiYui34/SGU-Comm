// Drives a browser through WebDriver for the voice test (see run.sh): join
// as a guest, replace the microphone with an oscillator, join the
// "General" voice channel, and talk in spurts with pauses. voicecheck
// listens and judges the packets.
//
//   node drive.mjs <browser> <webdriver URL> <app URL>
//
// No dependencies: plain W3C WebDriver over fetch.

const [browser, driverUrl, appUrl] = process.argv.slice(2);
const SPURTS = 4;
const ELEMENT = "element-6066-11e4-a52e-4f735466cecf";

const capabilities = {
  chrome: {
    browserName: "chrome",
    "goog:chromeOptions": { args: ["--headless=new", "--autoplay-policy=no-user-gesture-required", "--use-fake-ui-for-media-stream"] },
  },
  firefox: {
    browserName: "firefox",
    "moz:firefoxOptions": {
      args: ["-headless"],
      prefs: { "media.navigator.permission.disabled": true, "media.autoplay.default": 0, "media.autoplay.blocking_policy": 0 },
    },
  },
  safari: { browserName: "safari" },
}[browser];
if (!capabilities) throw new Error(`unknown browser ${browser}`);

const call = async (method, path, body) => {
  const res = await fetch(driverUrl + path, {
    method,
    headers: { "content-type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const json = await res.json();
  if (!res.ok) throw new Error(`${method} ${path}: ${JSON.stringify(json.value)}`);
  return json.value;
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// The microphone: an oscillator behind a gain the test turns up and down.
// Devices are stubbed too (headless Chrome crashes on enumerateDevices).
const stubMicrophone = `
  const md = navigator.mediaDevices;
  md.enumerateDevices = async () => [];
  md.getUserMedia = async () => {
    const c = new AudioContext({ sampleRate: 48000 });
    const o = c.createOscillator();
    o.frequency.value = 440;
    const g = c.createGain();
    g.gain.value = 0;
    window.__mic = g;
    const d = c.createMediaStreamDestination();
    o.connect(g);
    g.connect(d);
    o.start();
    await c.resume();
    return d.stream;
  };`;

const { sessionId } = await call("POST", "/session", { capabilities: { alwaysMatch: capabilities } });
const s = `/session/${sessionId}`;
const run = (script, args = []) => call("POST", `${s}/execute/sync`, { script, args });
const waitFor = async (what, script, timeout = 20_000) => {
  for (const end = Date.now() + timeout; Date.now() < end; await sleep(250)) {
    const found = await run(script);
    if (found) return found;
  }
  throw new Error(`timed out waiting for ${what}`);
};
const click = (el) => call("POST", `${s}/element/${el[ELEMENT]}/click`, {});

try {
  await call("POST", `${s}/url`, { url: appUrl });
  const name = await waitFor("the guest form", `return document.querySelector("input[required]")`);
  await call("POST", `${s}/element/${name[ELEMENT]}/value`, { text: `${browser}${Math.floor(Math.random() * 1000)}` });
  await click(await waitFor("the join button", `return document.querySelector("button[type=submit]")`));
  const general = await waitFor(
    "the General voice channel",
    `return [...document.querySelectorAll("nav button")].find((b) => /^\\s*General/.test(b.textContent)) ?? null`,
  );
  await run(stubMicrophone);
  await click(general); // a real click: a user gesture for audio
  await waitFor("the microphone", `return !!window.__mic`);
  console.log("voice:", await run(`return document.querySelector('[aria-label="Your status"]')?.textContent.replace(/\\s+/g, " ")`));
  console.log("audio:", await run(`return window.__mic.context.state`));
  await sleep(2000);
  for (let i = 0; i < SPURTS; i++) {
    await run(`window.__mic.gain.value = 0.5`);
    await sleep(1500);
    await run(`window.__mic.gain.value = 0`);
    await sleep(2000);
  }
  await sleep(1000);
} finally {
  await call("DELETE", s).catch(() => {});
}
