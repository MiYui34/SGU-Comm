// Playout timing for one speaker, in whole samples (no float drift at chunk
// boundaries). Voice comes over TCP, so packets are never lost or reordered,
// only late (the desktop app's UDP aside). Each chunk plays right after the
// previous one. A new or stalled stream starts after `startDelay`, which
// absorbs network jitter. When the queue is longer than `maxLatency` (a
// burst after a stall, or clock drift), the chunk is dropped (null), so
// latency cannot grow and audio never overlaps.
export type PlayoutConfig = { startDelay: number; maxLatency: number };

// A new stream, or one whose queue ran dry: the next chunk waits
// `startDelay`.
export const startsAnew = (prevEnd: number | null, now: number) => prevEnd === null || prevEnd < Math.round(now);

export const nextStart = (
  prevEnd: number | null,
  now: number,
  cfg: PlayoutConfig,
): number | null => {
  const t = Math.round(now);
  if (prevEnd === null || startsAnew(prevEnd, now)) return t + cfg.startDelay;
  return prevEnd > t + cfg.maxLatency ? null : prevEnd;
};

// A longer jump in the frame numbers is taken for a new talk spurt whose
// terminator did not come (lost, or a client without them), not for lost
// audio.
const MAX_GAP_US = 200_000;

// For the connection stats: what happened to one decoded chunk. expected:
// where the speaker's stream goes on (µs, null: a new talk spurt);
// timestamp and duration in µs; start (see nextStart), prevEnd and now in
// samples. A chunk that goes on the stream but found its queue run dry,
// was dropped, or came after a newer one is late. A jump forward means
// chunks went missing (only over UDP, in the desktop app).
export const chunkOutcome = (c: {
  expected: number | null;
  timestamp: number;
  duration: number;
  start: number | null;
  prevEnd: number | null;
  now: number;
}): { late: boolean; lost: number } => {
  const gap = c.expected === null ? 0 : c.timestamp - c.expected;
  const lost = gap > c.duration / 2 && gap <= MAX_GAP_US ? Math.round(gap / c.duration) : 0;
  const goesOn = c.expected !== null && Math.abs(gap) <= c.duration / 2;
  const ranDry = goesOn && c.prevEnd !== null && c.prevEnd < Math.round(c.now);
  return { late: c.start === null || gap < -c.duration / 2 || ranDry, lost };
};
