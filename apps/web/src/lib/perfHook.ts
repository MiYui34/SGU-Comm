import type { Counters } from "./connectionQuality";

// Dev hook for tests/perf: measure.mjs sets globalThis.__kallistiPerf before
// the app loads and samples the playback counters from it. Inert otherwise.
type Hook = { playback?: () => Counters };

export const exposePlayback = (stats: () => Counters, scope: { __kallistiPerf?: unknown } = globalThis as { __kallistiPerf?: unknown }) => {
  const hook = scope.__kallistiPerf as Hook | undefined;
  if (hook) hook.playback = stats;
};
