import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createLongPress } from "../src/lib/longPress";

describe("createLongPress", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("fires after holding still", () => {
    const fired = vi.fn();
    const p = createLongPress(fired);
    p.start(10, 20);
    vi.advanceTimersByTime(499);
    expect(fired).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(fired).toHaveBeenCalledWith(10, 20);
    expect(p.end()).toBe(true); // the click after it is swallowed
  });

  it("does not fire when released early or moved (scrolling)", () => {
    const fired = vi.fn();
    const p = createLongPress(fired);
    p.start(0, 0);
    vi.advanceTimersByTime(300);
    expect(p.end()).toBe(false);
    p.start(0, 0);
    p.move(0, 30);
    vi.advanceTimersByTime(600);
    expect(fired).not.toHaveBeenCalled();
  });

  it("leaves it to the browser when it sends its own context menu", () => {
    const fired = vi.fn();
    const p = createLongPress(fired);
    p.start(0, 0);
    p.native();
    vi.advanceTimersByTime(600);
    expect(fired).not.toHaveBeenCalled();
  });
});
