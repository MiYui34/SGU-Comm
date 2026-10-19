import { describe, expect, it } from "vitest";
import { exposePlayback } from "../src/lib/perfHook";

describe("exposePlayback", () => {
  const stats = () => ({ good: 3, late: 1, lost: 0 });

  it("hands the counters to a hook the page set up", () => {
    const scope: { __kallistiPerf?: { playback?: () => unknown } } = { __kallistiPerf: {} };
    exposePlayback(stats, scope);
    expect(scope.__kallistiPerf?.playback?.()).toEqual({ good: 3, late: 1, lost: 0 });
  });

  it("does nothing without one", () => {
    const scope: { __kallistiPerf?: unknown } = {};
    exposePlayback(stats, scope);
    expect(scope).toEqual({});
  });
});
