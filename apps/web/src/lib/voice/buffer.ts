import type { PlayoutConfig } from "./schedule";

// How long playback waits at the start of a stream (see schedule.ts): more
// absorbs worse jitter, at the cost of delay. Music gets at least 500 ms:
// delay does not matter for one-way music, gaps do.
export type PlaybackBuffer = "low" | "medium" | "high";
export const BUFFER_MS: Record<PlaybackBuffer, number> = { low: 60, medium: 120, high: 250 };
export const BUFFERS: readonly PlaybackBuffer[] = ["low", "medium", "high"];
const BUFFER_NAMES: Record<PlaybackBuffer, string> = { low: "低", medium: "中", high: "高" };
export const bufferLabel = (b: PlaybackBuffer) => `${BUFFER_NAMES[b]}（${BUFFER_MS[b]} ms）`;
const MUSIC_MS = 500;
// The queue may grow this much beyond the start delay before chunks are
// dropped (a burst after a stall, or clock drift).
const HEADROOM_MS = 240;
const SAMPLES_PER_MS = 48;

export const playoutFor = (buffer: PlaybackBuffer, music: boolean): PlayoutConfig => {
  const ms = music ? Math.max(MUSIC_MS, BUFFER_MS[buffer]) : BUFFER_MS[buffer];
  return { startDelay: ms * SAMPLES_PER_MS, maxLatency: (ms + HEADROOM_MS) * SAMPLES_PER_MS };
};
