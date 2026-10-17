import type { ChatMessage } from "@kallisti/protocol";
import { describe, expect, it } from "vitest";
import { chatItems, formatDay, formatTimestamp, groupMessages } from "../src/lib/chat";

const at = (y: number, m: number, d: number, h = 12, min = 0) => new Date(y, m - 1, d, h, min).getTime();
const now = at(2026, 10, 14, 16, 0);
const L = "en-US";

describe("formatTimestamp", () => {
  it("says Today and Yesterday for recent messages", () => {
    expect(formatTimestamp(at(2026, 10, 14, 15, 58), now, L)).toBe("Today at 3:58 PM");
    expect(formatTimestamp(at(2026, 10, 13, 9, 5), now, L)).toBe("Yesterday at 9:05 AM");
  });

  it("uses the date for older messages", () => {
    expect(formatTimestamp(at(2026, 10, 1, 9, 5), now, L)).toBe("10/1/2026 9:05 AM");
  });
});

describe("formatDay", () => {
  it("writes the full date", () => {
    expect(formatDay(at(2026, 10, 14), L)).toBe("October 14, 2026");
  });
});

const msg = (actor: number, t: number, text = "x"): ChatMessage => ({
  actor,
  actorName: `u${actor}`,
  channelIds: [0],
  treeIds: [],
  sessions: [],
  text,
  at: t,
    peer: null,
});

describe("groupMessages across midnight", () => {
  it("starts a new group on a new day", () => {
    const groups = groupMessages([msg(1, at(2026, 10, 13, 23, 59)), msg(1, at(2026, 10, 14, 0, 1))]);
    expect(groups).toHaveLength(2);
  });
});

describe("chatItems", () => {
  it("puts a date divider before the first group of each day", () => {
    const items = chatItems([
      msg(1, at(2026, 10, 13, 10)),
      msg(2, at(2026, 10, 13, 11)),
      msg(1, at(2026, 10, 14, 9)),
    ]);
    expect(items.map((i) => (i.kind === "group" ? `group:${i.group.actor}` : i.kind === "day" ? `day:${i.at}` : "blocked"))).toEqual([
      `day:${at(2026, 10, 13, 10)}`,
      "group:1",
      "group:2",
      `day:${at(2026, 10, 14, 9)}`,
      "group:1",
    ]);
  });
});

describe("chatItems with blocked users", () => {
  it("puts a blocked user's groups in a row into one item", () => {
    const named = (name: string, h: number, min = 0) => ({ ...msg(1, at(2026, 10, 13, h, min)), actor: null, actorName: name });
    const items = chatItems(
      [named("eve", 9), named("eve", 10), named("mallory", 11), named("bob", 12), named("eve", 13)],
      (name) => name === "eve" || name === "mallory",
    );
    expect(items.map((i) => (i.kind === "blocked" ? i.groups.map((g) => g.actorName).join("+") : i.kind))).toEqual([
      "day",
      "eve+eve+mallory",
      "group",
      "eve",
    ]);
  });

  it("keeps the days apart", () => {
    const eve = (d: number) => ({ ...msg(1, at(2026, 10, d)), actor: null, actorName: "eve" });
    expect(chatItems([eve(13), eve(14)], () => true).map((i) => i.kind)).toEqual(["day", "blocked", "day", "blocked"]);
  });
});

describe("groupMessages by name", () => {
  it("does not merge different authors that share no session", () => {
    const a = { ...msg(1, at(2026, 10, 14, 9)), actor: null, actorName: "alice" };
    const b = { ...msg(1, at(2026, 10, 14, 9, 1)), actor: null, actorName: "bob" };
    expect(groupMessages([a, b])).toHaveLength(2);
  });
});
