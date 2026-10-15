import { describe, expect, it } from "vitest";
import { DEFAULT_SETTINGS, keyLabel, meterPercent, parseSettings, pttAction, pttBlocksDefault } from "../src/lib/settings";

describe("parseSettings", () => {
  it("uses defaults for missing or broken input", () => {
    expect(parseSettings(null)).toEqual(DEFAULT_SETTINGS);
    expect(parseSettings("not json")).toEqual(DEFAULT_SETTINGS);
    expect(parseSettings("[1,2]")).toEqual(DEFAULT_SETTINGS);
  });

  it("keeps valid values", () => {
    const stored = {
      mode: "ptt",
      pttKey: "KeyV",
      vadThresholdDb: -30,
      inputDeviceId: "mic-2",
      outputDeviceId: "speakers-1",
      notifications: true,
      closeMicWhenMuted: true,
      noiseSuppression: "strong",
    };
    expect(parseSettings(JSON.stringify(stored))).toEqual(stored);
  });

  it("replaces invalid values one by one", () => {
    const parsed = parseSettings(
      JSON.stringify({ mode: "shout", pttKey: 5, vadThresholdDb: -300, inputDeviceId: "mic-2", notifications: "yes", closeMicWhenMuted: 1, noiseSuppression: "max" }),
    );
    expect(parsed).toEqual({ ...DEFAULT_SETTINGS, inputDeviceId: "mic-2" });
  });

  it("clamps the threshold to the slider range", () => {
    expect(parseSettings(JSON.stringify({ vadThresholdDb: 10 })).vadThresholdDb).toBe(0);
  });
});

describe("pttAction", () => {
  const key = "Backquote";
  const ev = (code: string, extra: { repeat?: boolean; editable?: boolean } = {}) => ({
    code,
    repeat: extra.repeat ?? false,
    editable: extra.editable ?? false,
  });

  it("presses and releases on the configured key", () => {
    expect(pttAction("keydown", ev(key), key)).toBe("down");
    expect(pttAction("keyup", ev(key), key)).toBe("up");
  });

  it("ignores other keys and key repeat", () => {
    expect(pttAction("keydown", ev("KeyA"), key)).toBeNull();
    expect(pttAction("keydown", ev(key, { repeat: true }), key)).toBeNull();
  });

  it("does not press while typing, but always releases", () => {
    expect(pttAction("keydown", ev(key, { editable: true }), key)).toBeNull();
    expect(pttAction("keyup", ev(key, { editable: true }), key)).toBe("up");
  });
});

describe("keyLabel", () => {
  it("makes key codes readable", () => {
    expect(keyLabel("KeyV")).toBe("V");
    expect(keyLabel("Digit4")).toBe("4");
    expect(keyLabel("Backquote")).toBe("`");
    expect(keyLabel("ShiftLeft")).toBe("Left Shift");
    expect(keyLabel("F13")).toBe("F13");
  });
});

describe("meterPercent", () => {
  it("maps -80..0 dB to 0..100", () => {
    expect(meterPercent(-80)).toBe(0);
    expect(meterPercent(-40)).toBe(50);
    expect(meterPercent(0)).toBe(100);
    expect(meterPercent(-100)).toBe(0);
    expect(meterPercent(6)).toBe(100);
  });
});

describe("pttBlocksDefault", () => {
  const ev = (code: string, editable = false) => ({ code, repeat: false, editable });

  it("blocks the PTT key outside text fields, also on keyup and repeat", () => {
    expect(pttBlocksDefault(ev("Space"), "Space")).toBe(true);
    expect(pttBlocksDefault({ ...ev("Space"), repeat: true }, "Space")).toBe(true);
  });

  it("never blocks typing or other keys", () => {
    expect(pttBlocksDefault(ev("Space", true), "Space")).toBe(false);
    expect(pttBlocksDefault(ev("Enter"), "Space")).toBe(false);
  });
});
