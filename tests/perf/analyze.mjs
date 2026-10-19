// Summarizes measure.mjs output files (the first 10 s after the socket
// opened, and the first 10 s of voice).
//
//   node analyze.mjs [--short] <out.json>...
//
// --short: one table row per file (see README.md for the columns).
import { readFileSync } from "node:fs";
import { basename } from "node:path";

const NAMES = { 1: "voice", 3: "ping", 5: "sync", 7: "channel", 9: "userstate", 15: "crypt", 24: "config", 20: "perms", 21: "codec" };
const args = process.argv.slice(2);
const short = args[0] === "--short";
const files = short ? args.slice(1) : args;
if (short) console.log(["run".padEnd(20), "voice s", "late", "last late s", "blob KB", "max gap ms", "long ms"].join(" | "));
for (const file of files) {
  const label = basename(file, ".json");
  const w = JSON.parse(readFileSync(file, "utf8"));
  const rel = (t) => (t - w.t0) / 1000;
  const kinds = {};
  const sec = {};
  for (const [t, k, b] of w.frames) {
    if (rel(t) > 10) continue;
    const name = k.startsWith("mumble:") ? (NAMES[k.slice(7)] ?? k) : k;
    const kind = name === "userstate" && b > 1000 ? "avatar-blob" : name;
    kinds[kind] ??= { n: 0, bytes: 0, first: rel(t), last: 0 };
    Object.assign(kinds[kind], { n: kinds[kind].n + 1, bytes: kinds[kind].bytes + b, last: rel(t) });
    const s = Math.floor(rel(t));
    sec[s] ??= {};
    sec[s][kind] = (sec[s][kind] ?? 0) + b;
  }
  // Voice arrival gaps (20 ms each when smooth).
  const voice = w.frames.filter(([, k]) => k === "mumble:1").map(([t]) => t);
  const gaps = voice.slice(1).map((t, i) => t - voice[i]);
  const joined = rel(w.joined);
  const firstVoice = voice.length ? rel(voice[0]) : null;
  const fv = firstVoice ?? 0;
  const maxGap = (from, to) => Math.max(0, ...gaps.filter((_, i) => rel(voice[i + 1]) >= from && rel(voice[i + 1]) < to));
  // Playback counters [s, good, late, lost]; the last rise of late marks
  // the end of the jerky start.
  const st = w.stats.map(([t, g, l, lo]) => [rel(t), g, l, lo]);
  const at = (x) => st.filter((s) => s[0] <= x).at(-1) ?? [x, 0, 0, 0];
  let lastLate = null;
  for (let i = 1; i < st.length; i++) if (st[i][2] > st[i - 1][2]) lastLate = st[i][0];
  const end = st.at(-1);
  const in10 = at(fv + 10);
  const longs = w.long.filter(([t]) => rel(t) <= 10);
  const longTotal = longs.reduce((a, [, d]) => a + d, 0);
  const sinceVoice = lastLate === null ? "-" : (lastLate - fv).toFixed(2);
  if (short) {
    const voiceAt = firstVoice === null ? "-" : (fv - joined).toFixed(2);
    console.log([label.padEnd(20), voiceAt, in10[2], sinceVoice, ((kinds["avatar-blob"]?.bytes ?? 0) / 1024).toFixed(0), Math.max(0, ...gaps).toFixed(0), longTotal.toFixed(0)].join(" | "));
    continue;
  }
  console.log(`\n== ${label}: joined at ${joined.toFixed(2)} s, first voice ${firstVoice?.toFixed(2) ?? "-"} s`);
  console.log(`late in first 10 s of voice: ${in10[2]} of ${in10[1] + in10[2]} chunks; last late chunk ${sinceVoice} s after first voice; total late ${end?.[2]} good ${end?.[1]}`);
  console.log(`voice gaps: max ${Math.max(0, ...gaps).toFixed(0)} ms; per second max gap:`, Array.from({ length: 10 }, (_, s) => maxGap(fv + s, fv + s + 1).toFixed(0)).join(" "));
  console.log(`long tasks in 10 s: ${longs.length}, total ${longTotal.toFixed(0)} ms, max ${Math.max(0, ...longs.map(([, d]) => d)).toFixed(0)} ms`);
  console.log("per kind (first 10 s): kind n bytes first last");
  for (const [k, v] of Object.entries(kinds).sort((a, b) => b[1].bytes - a[1].bytes))
    console.log(`  ${k.padEnd(18)} ${String(v.n).padStart(5)} ${String(v.bytes).padStart(9)}  ${v.first.toFixed(2)}  ${v.last.toFixed(2)}`);
  console.log("bytes per second (KB): s total voice avatar json");
  for (let s = 0; s < 10; s++) {
    const o = sec[s] ?? {};
    const tot = Object.values(o).reduce((a, b) => a + b, 0);
    const json = Object.entries(o)
      .filter(([k]) => k.startsWith("json"))
      .reduce((a, [, b]) => a + b, 0);
    console.log(`  ${s} ${(tot / 1024).toFixed(0).padStart(6)} ${((o.voice ?? 0) / 1024).toFixed(1).padStart(6)} ${((o["avatar-blob"] ?? 0) / 1024).toFixed(0).padStart(6)} ${(json / 1024).toFixed(0).padStart(6)}`);
  }
  console.log("late counter by second after first voice:", Array.from({ length: 11 }, (_, s) => at(fv + s)[2]).join(" "));
}
