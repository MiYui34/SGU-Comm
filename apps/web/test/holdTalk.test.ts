import { describe, expect, it } from "vitest";
import { createHoldTalk } from "../src/lib/holdTalk";

describe("createHoldTalk", () => {
  const recorder = () => {
    const calls: boolean[] = [];
    return { calls, talk: (down: boolean) => void calls.push(down) };
  };

  it("presses and releases once", () => {
    const a = recorder();
    const hold = createHoldTalk();
    hold.press(a.talk);
    hold.release();
    hold.release();
    expect(a.calls).toEqual([true, false]);
  });

  it("releases with the callback that took the press", () => {
    const [a, b] = [recorder(), recorder()];
    const hold = createHoldTalk();
    hold.press(a.talk);
    // The button went away (or got a new callback) while held.
    hold.release();
    expect(a.calls).toEqual([true, false]);
    expect(b.calls).toEqual([]);
  });

  it("releases an earlier press before a new one", () => {
    const [a, b] = [recorder(), recorder()];
    const hold = createHoldTalk();
    hold.press(a.talk);
    hold.press(b.talk);
    expect(a.calls).toEqual([true, false]);
    expect(b.calls).toEqual([true]);
  });

  it("does nothing on a release without a press", () => {
    const hold = createHoldTalk();
    expect(() => hold.release()).not.toThrow();
  });
});
