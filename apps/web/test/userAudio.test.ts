import type { ChatMessage } from "@mumble-discord/protocol";
import { describe, expect, it } from "vitest";
import { privateCountsFrom, unreadCounts } from "../src/lib/chat";
import {
  DEFAULT_USER_AUDIO,
  gainOf,
  parseUserAudio,
  updateUserAudio,
  userAudioFor,
} from "../src/lib/userAudio";

describe("parseUserAudio", () => {
  it("returns an empty map for missing or broken input", () => {
    expect(parseUserAudio(null)).toEqual({});
    expect(parseUserAudio("nope")).toEqual({});
    expect(parseUserAudio("[]")).toEqual({});
  });

  it("keeps valid entries, clamps volume and drops broken entries", () => {
    const raw = JSON.stringify({
      alice: { volume: 1.5, muted: false },
      bob: { volume: 9, muted: true },
      carl: "loud",
      dave: { volume: "x", muted: 1 },
    });
    expect(parseUserAudio(raw)).toEqual({
      alice: { volume: 1.5, muted: false },
      bob: { volume: 2, muted: true },
      dave: DEFAULT_USER_AUDIO,
    });
  });
});

describe("updateUserAudio", () => {
  it("sets values for one user", () => {
    const prefs = updateUserAudio({}, "alice", { volume: 0.5 });
    expect(userAudioFor(prefs, "alice")).toEqual({ volume: 0.5, muted: false });
    expect(userAudioFor(prefs, "bob")).toEqual(DEFAULT_USER_AUDIO);
  });

  it("removes entries that are back at the default", () => {
    const prefs = updateUserAudio({ alice: { volume: 0.5, muted: false } }, "alice", {
      volume: 1,
    });
    expect(prefs).toEqual({});
  });

  it("clamps the volume", () => {
    expect(updateUserAudio({}, "a", { volume: -1 }).a?.volume).toBe(0);
  });
});

describe("gainOf", () => {
  it("is the volume, or 0 when muted", () => {
    expect(gainOf({ volume: 1.5, muted: false })).toBe(1.5);
    expect(gainOf({ volume: 1.5, muted: true })).toBe(0);
  });
});

describe("privateCountsFrom", () => {
  const msg = (actor: number | null, sessions: number[], channelIds: number[] = []): ChatMessage => ({
    actor,
    actorName: null,
    channelIds,
    treeIds: [],
    sessions,
    text: "",
    at: 0,
  });

  it("counts private messages to us per sender", () => {
    const counts = privateCountsFrom(
      [msg(2, [1]), msg(2, [1]), msg(3, [1]), msg(1, [2]), msg(2, [], [0]), msg(4, [5])],
      1,
    );
    expect(counts).toEqual({ 2: 2, 3: 1 });
  });
});

describe("unreadCounts", () => {
  it("subtracts what was read and drops users with nothing unread", () => {
    expect(unreadCounts({ 2: 3, 3: 1, 4: 2 }, { 2: 1, 3: 1 })).toEqual({ 2: 2, 4: 2 });
  });
});
