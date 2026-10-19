import { describe, expect, it } from "vitest";
import { mutedIds, nothingIds, notifies, parseChannelPrefs, prefOf, setChannelPref, withoutMuted } from "../src/lib/channelPrefs";

const A = "https://a";

describe("channel settings", () => {
  it("reads stored data, also the old list of muted channels", () => {
    expect(parseChannelPrefs(null)).toEqual({});
    expect(parseChannelPrefs("nope")).toEqual({});
    expect(parseChannelPrefs(JSON.stringify({ [A]: [1, "x", 2], b: "no" }))).toEqual({ [A]: { 1: { mutedUntil: 0 }, 2: { mutedUntil: 0 } } });
    expect(parseChannelPrefs(JSON.stringify({ [A]: { 3: { level: "all", mutedUntil: 99 }, 4: { level: "loud" }, x: {} } }))).toEqual({
      [A]: { 3: { level: "all", mutedUntil: 99 } },
    });
  });

  it("sets per server and channel, and forgets defaults", () => {
    let prefs = setChannelPref({}, A, 3, { mutedUntil: 0 });
    prefs = setChannelPref(prefs, "https://b", 3, { level: "nothing" });
    expect(prefOf(prefs, A, 3)).toEqual({ mutedUntil: 0 });
    prefs = setChannelPref(prefs, A, 3, { mutedUntil: undefined });
    expect(prefs[A]).toEqual({});
    expect(prefOf(prefs, "https://b", 3)).toEqual({ level: "nothing" });
    expect(setChannelPref(prefs, "https://b", 3, { level: "mentions" })["https://b"]).toEqual({});
  });

  it("mutes until a time, or for ever (0)", () => {
    const prefs = { [A]: { 1: { mutedUntil: 0 }, 2: { mutedUntil: 1000 }, 3: { mutedUntil: 500 }, 4: { level: "all" as const } } };
    expect(mutedIds(prefs, A, 800)).toEqual(new Set([1, 2]));
    expect(withoutMuted(new Set([1, 2, 3, 4]), new Set([1, 2]))).toEqual(new Set([3, 4]));
  });

  it("lists the channels set to nothing (no push notifications)", () => {
    const prefs = { [A]: { 1: { level: "nothing" as const }, 2: { mutedUntil: 0 }, 5: { level: "nothing" as const, mutedUntil: 0 } }, b: { 3: { level: "nothing" as const } } };
    expect(nothingIds(prefs, A)).toEqual([1, 5]);
    expect(nothingIds(prefs, "c")).toEqual([]);
  });

  it("notifies by the level; muted: only mentions; nothing: never", () => {
    expect(notifies({}, false, 0)).toBe(false); // only @mentions by default
    expect(notifies({}, true, 0)).toBe(true);
    expect(notifies({ level: "all" }, false, 0)).toBe(true);
    expect(notifies({ level: "all", mutedUntil: 0 }, false, 0)).toBe(false);
    expect(notifies({ level: "all", mutedUntil: 0 }, true, 0)).toBe(true);
    expect(notifies({ level: "all", mutedUntil: 10 }, false, 20)).toBe(true); // mute is over
    expect(notifies({ level: "nothing" }, true, 0)).toBe(false);
  });
});
