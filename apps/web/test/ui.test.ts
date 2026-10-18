import type { ChatMessage } from "@kallisti/protocol";
import { describe, expect, it } from "vitest";
import { avatarColor, initials } from "../src/lib/avatar";
import { chatItems, groupMessages, itemKey, messageKey } from "../src/lib/chat";
import { toggleDeaf, toggleMute } from "../src/lib/voice";

describe("avatar", () => {
  it("uses the first letter in upper case", () => {
    expect(initials("alice")).toBe("A");
    expect(initials("  ")).toBe("?");
    expect(initials("élodie")).toBe("É");
  });

  it("gives the same color for the same name", () => {
    expect(avatarColor("alice")).toBe(avatarColor("alice"));
    expect(avatarColor("alice")).toMatch(/^#[0-9a-f]{6}$/);
  });

  it("spreads names over different colors", () => {
    const colors = new Set(["alice", "bob", "carol", "dave", "eve", "frank"].map(avatarColor));
    expect(colors.size).toBeGreaterThan(2);
  });
});

describe("voice toggles", () => {
  it("toggles mute", () => {
    expect(toggleMute({ selfMute: false, selfDeaf: false })).toEqual({
      selfMute: true,
      selfDeaf: false,
    });
  });

  it("undeafens when unmuting while deafened", () => {
    expect(toggleMute({ selfMute: true, selfDeaf: true })).toEqual({
      selfMute: false,
      selfDeaf: false,
    });
  });

  it("deafen also mutes", () => {
    expect(toggleDeaf({ selfMute: false, selfDeaf: false })).toEqual({
      selfMute: true,
      selfDeaf: true,
    });
  });

  it("undeafen also unmutes", () => {
    expect(toggleDeaf({ selfMute: true, selfDeaf: true })).toEqual({
      selfMute: false,
      selfDeaf: false,
    });
  });
});

describe("groupMessages", () => {
  const msg = (actor: number | null, at: number, text = "x"): ChatMessage => ({
    actor,
    actorName: actor === null ? null : `user${actor}`,
    channelIds: [0],
    treeIds: [],
    sessions: [],
    text,
    at,
    peer: null,
  });
  const MIN = 60_000;

  it("groups consecutive messages from one sender within 5 minutes", () => {
    const groups = groupMessages([
      msg(1, 0, "a"),
      msg(1, 2 * MIN, "b"),
      msg(2, 3 * MIN, "c"),
      msg(2, 9 * MIN, "d"),
    ]);
    expect(groups.map((g) => g.messages.map((m) => m.text))).toEqual([["a", "b"], ["c"], ["d"]]);
    expect(groups[0]).toMatchObject({ actor: 1, actorName: "user1", at: 0 });
  });

  it("does not group server messages with user messages", () => {
    const groups = groupMessages([msg(null, 0), msg(1, 1)]);
    expect(groups).toHaveLength(2);
  });
});

describe("messageKey and itemKey", () => {
  const line = (text: string, id?: number): ChatMessage & { source?: { id: number; text: string } } => ({
    actor: 1,
    actorName: "user1",
    channelIds: [0],
    treeIds: [],
    sessions: [],
    text,
    at: 0,
    peer: null,
    ...(id !== undefined && { source: { id, text } }),
  });

  it("keys a stored message by its id: a new object (a reaction) keeps the key", () => {
    expect(messageKey(line("a", 7))).toBe(messageKey({ ...line("a", 7), text: "a!" }));
    expect(messageKey(line("a", 7))).not.toBe(messageKey(line("a", 8)));
  });

  it("keys a Mumble chat line by its object, the same each time", () => {
    const [a, b] = [line("hi"), line("hi")];
    expect(messageKey(a)).toBe(messageKey(a));
    expect(messageKey(a)).not.toBe(messageKey(b));
  });

  it("keys groups by their first message and days by their time", () => {
    const m = line("a", 7);
    const items = chatItems([m, line("b", 8)]);
    expect(items.map(itemKey)).toEqual([`day-${m.at}`, messageKey(m)]);
    const again = chatItems([{ ...m, text: "a, reacted" }, line("b", 8)]);
    expect(again.map(itemKey)).toEqual(items.map(itemKey));
  });
});
