// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createLongPress, hold } from "../src/lib/longPress";

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

describe("hold", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  const touch = (node: HTMLElement, type: string, x = 0, y = 0) => {
    const e = new Event(type, { bubbles: true, cancelable: true });
    Object.assign(e, { touches: [{ clientX: x, clientY: y }], changedTouches: [{ clientX: x, clientY: y }] });
    node.dispatchEvent(e);
    return e;
  };

  it("runs the newest callback on a long press, and swallows the click after it", () => {
    const node = document.createElement("div");
    const first = vi.fn();
    const second = vi.fn();
    const action = hold(node, first);
    action.update(second);
    touch(node, "touchstart");
    vi.advanceTimersByTime(500);
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledOnce();
    expect(touch(node, "touchend").defaultPrevented).toBe(true);
    action.destroy();
    touch(node, "touchstart");
    vi.advanceTimersByTime(500);
    expect(second).toHaveBeenCalledOnce();
  });

  it("leaves text fields and media controls alone", () => {
    const node = document.createElement("div");
    const field = node.appendChild(document.createElement("textarea"));
    const video = node.appendChild(document.createElement("video"));
    const onHold = vi.fn();
    hold(node, onHold);
    touch(field, "touchstart");
    vi.advanceTimersByTime(500);
    touch(video, "touchstart");
    vi.advanceTimersByTime(500);
    expect(onHold).not.toHaveBeenCalled();
  });
});
