// Summarizes stream.mjs output files: the streamer's encoder and upload,
// and per viewer the arrival and drawing of pictures and the scheduling of
// sound, from WARMUP seconds after the watch click on (see README.md).
//
//   node stream-analyze.mjs <out.json>...
import { readFileSync } from "node:fs";
import { basename } from "node:path";

const WARMUP = 3_000;
const FREEZE = 200; // ms without a new picture
const IN_FLIGHT = 2_000; // the last pictures sent may not be drawn yet
const KEY = 1;
const SOUND = 2;

const pct = (xs, p) => (xs.length ? [...xs].sort((a, b) => a - b)[Math.min(xs.length - 1, Math.floor(xs.length * p))] : 0);
const gaps = (ts) => ts.slice(1).map((t, i) => t - ts[i]);
const f0 = (x) => x.toFixed(0);
const f1 = (x) => x.toFixed(1);

export const summarize = (r) => {
  const from = r.watched + WARMUP;
  const to = r.watched + r.duration * 1000;
  const secs = (to - from) / 1000;
  const inWindow = ([t]) => t >= from && t < to;
  const s = r.streamer;
  const pictures = s.sent.filter((f) => !(f[1] & SOUND) && inWindow(f));
  const settled = pictures.filter(([t]) => t < to - IN_FLIGHT);
  const keys = pictures.filter((f) => f[1] & KEY);
  const deltas = pictures.filter((f) => !(f[1] & KEY));
  const streamer = {
    encodes: s.encodes.filter(inWindow).length / secs,
    sentFps: pictures.length / secs,
    mbits: (pictures.reduce((a, f) => a + f[3], 0) * 8) / secs / 1e6,
    keys: keys.length,
    keyKB: keys.length ? Math.max(...keys.map((f) => f[3])) / 1024 : 0,
    deltaKB: deltas.length ? deltas.reduce((a, f) => a + f[3], 0) / deltas.length / 1024 : 0,
    maxQueue: Math.max(0, ...s.encodes.filter(inWindow).map((e) => e[1])),
    backedUp: pictures.filter((f) => f[4] > 1_500_000).length,
    skipped: s.captured - s.encodes.length, // captured, not encoded (whole run)
    asked: s.texts.filter((x) => x[1] === "keyframe" && inWindow(x)).length, // by the server
  };
  const viewers = r.viewers.map((v) => {
    const got = v.got.filter((f) => !(f[1] & SOUND));
    const gotTs = new Set(got.map((f) => f[2]));
    const drawn = new Map();
    for (const [t, ts] of v.draws) if (!drawn.has(ts)) drawn.set(ts, t);
    const draws = v.draws.filter(inWindow).map(([t]) => t);
    const drawGaps = gaps(draws);
    const freezes = drawGaps.filter((g) => g > FREEZE);
    // Runs of sent pictures that were not drawn: the time until the next
    // drawn one (a wait for a keyframe, or skipped late pictures).
    const waits = [];
    let first = null;
    for (const f of settled) {
      if (!drawn.has(f[2])) first ??= f;
      else if (first) waits.push((f[2] - first[2]) / 1000), (first = null);
    }
    if (first) waits.push((settled.at(-1)[2] - first[2]) / 1000);
    const lat = settled.filter((f) => drawn.has(f[2])).map((f) => drawn.get(f[2]) - f[0]);
    // Sound: arrived packets, scheduled ones, and jumps in the schedule (a
    // gap or a new start of the clock).
    const soundsGot = v.got.filter((f) => f[1] & SOUND && inWindow(f)).length;
    const sched = v.sounds.filter(inWindow);
    const jumps = gaps(sched.map((x) => x[1])).filter((g) => Math.abs(g - 0.02) > 0.005).length;
    // Picture against sound: the audio clock minus the stream time, when a
    // picture is drawn and when sound plays (positive: the picture late).
    const offsets = v.sounds.filter((x) => x[3] !== null);
    let k = 0;
    const skews = [];
    for (const [t, ts, clock] of v.draws.filter((d) => inWindow(d) && d[2] !== null)) {
      while (k + 1 < offsets.length && offsets[k + 1][0] <= t) k++;
      if (offsets[k]) skews.push((clock - ts / 1e6 - (offsets[k][1] - offsets[k][3] / 1e6)) * 1000);
    }
    const first0 = v.draws[0]?.[0];
    return {
      url: v.url,
      startS: first0 ? (first0 - v.watched) / 1000 : null,
      gotFps: got.filter(inWindow).length / secs,
      arrivalP95: pct(gaps(got.filter(inWindow).map((f) => f[0])), 0.95),
      arrivalMax: Math.max(0, ...gaps(got.filter(inWindow).map((f) => f[0]))),
      drawFps: draws.length / secs,
      drawP50: pct(drawGaps, 0.5),
      drawP95: pct(drawGaps, 0.95),
      drawMax: Math.max(0, ...drawGaps),
      freezes: freezes.length,
      frozenMs: freezes.reduce((a, g) => a + g, 0),
      lost: settled.filter((f) => !gotTs.has(f[2])).length,
      undrawn: settled.filter((f) => gotTs.has(f[2]) && !drawn.has(f[2])).length,
      waits: waits.filter((w) => w > 100).length,
      waitMax: Math.max(0, ...waits),
      latP50: pct(lat, 0.5),
      latP95: pct(lat, 0.95),
      soundsGot,
      soundsDropped: soundsGot - sched.length,
      soundJumps: jumps,
      skew: sched.length && skews.length ? `${f0(pct(skews, 0.05))}/${f0(pct(skews, 0.5))}/${f0(pct(skews, 0.95))}` : "-",
      keyAsks: v.texts.filter(([t, x]) => x === "> keyframe" && t >= from && t < to).length,
    };
  });
  return { streamer, viewers };
};

if (import.meta.url === `file://${process.argv[1]}`) {
  console.log(
    "run | viewer | start s | got fps | arr p95/max ms | draw fps | draw p50/p95/max ms | freezes (ms) | lost | undrawn | waits>100ms (max ms) | latency p50/p95 ms | sound got/dropped/jumps | key asks | a/v ms p5/p50/p95",
  );
  for (const file of process.argv.slice(2)) {
    const r = JSON.parse(readFileSync(file, "utf8"));
    const { streamer: s, viewers } = summarize(r);
    const label = basename(file, ".json");
    console.log(
      `${label} | streamer | encodes ${f1(s.encodes)}/s, sent ${f1(s.sentFps)} fps, ${s.mbits.toFixed(2)} Mbit/s, ${s.keys} keys (max ${f0(s.keyKB)} KB), delta ${f1(s.deltaKB)} KB, encode queue max ${s.maxQueue}, not encoded ${s.skipped}, sends with a full socket ${s.backedUp}, keyframe requests ${s.asked}`,
    );
    for (const v of viewers) {
      const via = new URL(v.url).port;
      console.log(
        [
          label,
          via,
          v.startS?.toFixed(2) ?? "-",
          f1(v.gotFps),
          `${f0(v.arrivalP95)}/${f0(v.arrivalMax)}`,
          f1(v.drawFps),
          `${f0(v.drawP50)}/${f0(v.drawP95)}/${f0(v.drawMax)}`,
          `${v.freezes} (${f0(v.frozenMs)})`,
          v.lost,
          v.undrawn,
          `${v.waits} (${f0(v.waitMax)})`,
          `${f0(v.latP50)}/${f0(v.latP95)}`,
          `${v.soundsGot}/${v.soundsDropped}/${v.soundJumps}`,
          v.keyAsks,
          v.skew,
        ].join(" | "),
      );
    }
  }
}
