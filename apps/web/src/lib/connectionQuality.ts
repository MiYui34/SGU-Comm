import { BUFFERS, type PlaybackBuffer } from "./voice/buffer";

// The connection info next to "Voice connected": ping and late or lost
// audio over the last 30 s, and a level from them.

export const WINDOW_MS = 30_000;

export type Level = "good" | "fair" | "poor";
// Audio chunks from all speakers since the connection started (see
// voice/schedule chunkOutcome).
export type Counters = { good: number; late: number; lost: number };

export const recent = <T extends { at: number }>(items: readonly T[], now: number): T[] =>
  items.filter((i) => now - i.at <= WINDOW_MS);

// The variance, not the spread: Mumble's Ping wants it (clients show the
// square root).
export const meanVariance = (values: readonly number[]): { mean: number; variance: number } | null => {
  if (!values.length) return null;
  const mean = values.reduce((a, b) => a + b, 0) / values.length;
  return { mean, variance: values.reduce((a, v) => a + (v - mean) ** 2, 0) / values.length };
};

// Late and lost audio in percent of all audio between two snapshots (null:
// no audio came).
const percentOf = (from: Counters, to: Counters, bad: number): number | null => {
  const total = to.good - from.good + (to.late - from.late) + (to.lost - from.lost);
  return total > 0 ? (bad / total) * 100 : null;
};
export const badPercent = (from: Counters, to: Counters) => percentOf(from, to, to.late - from.late + (to.lost - from.lost));
// Late only: what a larger playback buffer helps against.
export const latePercent = (from: Counters, to: Counters) => percentOf(from, to, to.late - from.late);

// Discord-like: at 100 ms people start to talk over each other, beyond
// 250 ms talking is hard. Late audio is a gap in the speech: a few in 100
// can pass, one in 12 is choppy. The worse of both counts.
const LIMITS: [Level, number, number][] = [
  ["good", 100, 2],
  ["fair", 250, 8],
];

// rtt: the mean ping in ms (null: none came back yet). bad: see badPercent.
export const qualityLevel = (rtt: number | null, bad: number | null): Level | null =>
  rtt === null ? null : (LIMITS.find(([, ms, pct]) => rtt < ms && (bad ?? 0) < pct)?.[0] ?? "poor");

// Late audio from "fair" on (see LIMITS): suggest the next playback
// buffer, if there is one.
export const biggerBuffer = (late: number | null, buffer: PlaybackBuffer): PlaybackBuffer | null =>
  late !== null && late >= LIMITS[0]![2] ? (BUFFERS[BUFFERS.indexOf(buffer) + 1] ?? null) : null;

export const connectionLabel = (level: Level | null, rtt: number | null) =>
  level === null || rtt === null ? "Connection: measuring" : `Connection: ${level}, ${Math.round(rtt)} ms`;

// The desktop app's bridge (see lib/desktop bridgeUrl) sends voice over UDP
// while UDP works; the page cannot see which.
export const transportOf = (url: string) =>
  url.startsWith("ws://127.0.0.1:") && url.includes("secret=") ? "UDP or TCP (desktop app)" : "WebSocket";
