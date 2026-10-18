import { describe, expect, it } from "vitest";
import { createHoldTalk } from "../src/lib/holdTalk";

describe("createHoldTalk", () => {
  const recorder = () => {
    const calls: boolean[] = [];
    return { calls, talk: (down: boolean) => void calls.push(down) };
  };

  // The release needs no callback: it always uses the one of the press,
  // also after the button went away.
  it("releases the press once, with its callback", () => {
    const a = recorder();
    const hold = createHoldTalk();
    hold.press(a.talk);
    hold.release();
    hold.release();
    expect(a.calls).toEqual([true, false]);
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
