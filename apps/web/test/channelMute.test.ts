import { describe, expect, it } from "vitest";
import { mutedOn, parseMuted, toggleMuted, withoutMuted } from "../src/lib/channelMute";

describe("muted channels", () => {
  it("reads stored data, dropping what is broken", () => {
    expect(parseMuted(null)).toEqual({});
    expect(parseMuted("nope")).toEqual({});
    expect(parseMuted(JSON.stringify({ "https://a": [1, "x", 2], b: "no" }))).toEqual({ "https://a": [1, 2] });
  });

  it("toggles per server", () => {
    let prefs = toggleMuted({}, "https://a", 3);
    prefs = toggleMuted(prefs, "https://b", 3);
    expect(mutedOn(prefs, "https://a")).toEqual(new Set([3]));
    prefs = toggleMuted(prefs, "https://a", 3);
    expect(mutedOn(prefs, "https://a")).toEqual(new Set());
    expect(mutedOn(prefs, "https://b")).toEqual(new Set([3]));
  });

  it("muted channels are not unread", () => {
    expect(withoutMuted(new Set([1, 2, 3]), new Set([2]))).toEqual(new Set([1, 3]));
  });
});
