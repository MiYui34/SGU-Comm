// Playout timing for one speaker, in whole samples (no float drift at chunk
// boundaries). Voice comes over TCP, so packets are never lost or reordered,
// only late. Each chunk plays right after the previous one. A new or stalled
// stream starts after `startDelay`, which absorbs network jitter. When the
// queue is longer than `maxLatency` (a burst after a stall, or clock drift),
// the chunk is dropped (null), so latency cannot grow and audio never overlaps.
export type PlayoutConfig = { startDelay: number; maxLatency: number };

export const nextStart = (
  prevEnd: number | null,
  now: number,
  cfg: PlayoutConfig,
): number | null => {
  const t = Math.round(now);
  if (prevEnd === null || prevEnd < t) return t + cfg.startDelay;
  return prevEnd > t + cfg.maxLatency ? null : prevEnd;
};
