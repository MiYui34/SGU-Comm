// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { watchAway } from "../src/lib/away";
import { AWAY_AFTER_MS } from "../src/lib/presence";

describe("watchAway", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("away after 10 minutes without input, back on input", () => {
    const changes: boolean[] = [];
    const stop = watchAway((away) => changes.push(away));
    vi.advanceTimersByTime(AWAY_AFTER_MS - 60_000);
    window.dispatchEvent(new Event("keydown"));
    vi.advanceTimersByTime(AWAY_AFTER_MS - 30_000);
    expect(changes).toEqual([]);
    vi.advanceTimersByTime(60_000);
    expect(changes).toEqual([true]);
    window.dispatchEvent(new Event("pointerdown"));
    expect(changes).toEqual([true, false]);
    stop();
    vi.advanceTimersByTime(2 * AWAY_AFTER_MS);
    expect(changes).toEqual([true, false]);
  });
});
