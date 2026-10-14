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
    expect(items.map((i) => (i.kind === "day" ? `day:${i.at}` : `group:${i.group.actor}`))).toEqual([
      `day:${at(2026, 10, 13, 10)}`,
      "group:1",
      "group:2",
      `day:${at(2026, 10, 14, 9)}`,
      "group:1",
    ]);
  });
});

describe("groupMessages by name", () => {
  it("does not merge different authors that share no session", () => {
    const a = { ...msg(1, at(2026, 10, 14, 9)), actor: null, actorName: "alice" };
    const b = { ...msg(1, at(2026, 10, 14, 9, 1)), actor: null, actorName: "bob" };
    expect(groupMessages([a, b])).toHaveLength(2);
  });
});
