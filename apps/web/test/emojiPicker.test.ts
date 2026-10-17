import { describe, expect, it } from "vitest";
import {
  addRecent,
  gridMove,
  normalEmoji,
  parseEmojiData,
  parseRecent,
  quickReactions,
  searchEmoji,
  withTone,
  type EmojiEntry,
} from "../src/lib/emojiPicker";
import { COMMON_EMOJI } from "../src/lib/reactions";

const raw = {
  groups: [
    { name: "Smileys & People", icon: "😀" },
    { name: "Food & Drink", icon: "🍔" },
  ],
  emoji: [
    ["😀", "grinning face", "face grin grinning", 0],
    ["👍", "thumbs up", "+1 good thumb thumbsup", 0, ["👍🏻", "👍🏼", "👍🏽", "👍🏾", "👍🏿"]],
    ["👎", "thumbs down", "-1 bad thumb", 0, ["👎🏻", "👎🏼", "👎🏽", "👎🏾", "👎🏿"]],
    ["🍔", "hamburger", "burger food", 1],
    ["🍕", "pizza", "cheese slice", 1],
  ],
};

describe("parseEmojiData", () => {
  it("reads entries and drops broken ones", () => {
    const data = parseEmojiData({ ...raw, emoji: [...raw.emoji, [1, 2], "x", ["🙂", "slightly smiling face", "", 7]] });
    expect(data.groups).toHaveLength(2);
    expect(data.emoji.map((e) => e.emoji)).toEqual(["😀", "👍", "👎", "🍔", "🍕"]);
    expect(data.emoji[1]).toEqual({ emoji: "👍", name: "thumbs up", keywords: "+1 good thumb thumbsup", group: 0, skins: raw.emoji[1]![4] });
  });

  it("gives an empty list for broken data", () => {
    expect(parseEmojiData(null)).toEqual({ groups: [], emoji: [] });
    expect(parseEmojiData({ groups: 3 })).toEqual({ groups: [], emoji: [] });
  });
});

describe("searchEmoji", () => {
  const { emoji } = parseEmojiData(raw);
  const names = (q: string) => searchEmoji(emoji, q).map((e) => e.name);

  it("finds words by their start, in names and keywords", () => {
    expect(names("thumb")).toEqual(["thumbs up", "thumbs down"]);
    expect(names("+1")).toEqual(["thumbs up"]);
    expect(names("burg")).toEqual(["hamburger"]);
    expect(names("ham")).toEqual(["hamburger"]);
  });

  it("needs every word, ignores case and colons", () => {
    expect(names("Thumbs D")).toEqual(["thumbs down"]);
    expect(names(":thumbsup:")).toEqual(["thumbs up"]);
    expect(names("thumb pizza")).toEqual([]);
  });

  it("puts names that start with the search first", () => {
    expect(names("g")).toEqual(["grinning face", "thumbs up"]);
  });

  it("gives nothing for an empty search", () => {
    expect(names("  ")).toEqual([]);
  });
});

describe("withTone", () => {
  const [, thumbs, , burger] = parseEmojiData(raw).emoji as [unknown, EmojiEntry, unknown, EmojiEntry];
  it("picks the skin tone when the emoji has them", () => {
    expect(withTone(thumbs, 0)).toBe("👍");
    expect(withTone(thumbs, 3)).toBe("👍🏽");
    expect(withTone(burger, 3)).toBe("🍔");
  });
});

describe("normalEmoji", () => {
  it("is the server's form: the fully qualified one", () => {
    const cases: [string, string][] = [
      ["❤", "❤\uFE0F"],
      ["❤\uFE0F", "❤\uFE0F"],
      ["👍\uFE0F", "👍"],
      ["✅\uFE0F", "✅"],
      ["🕳", "🕳\uFE0F"],
      ["✋\uFE0F🏻", "✋🏻"],
      ["⛹🏽\u200D♂", "⛹🏽\u200D♂\uFE0F"],
      ["#\u20E3", "#\uFE0F\u20E3"],
      ["🎓\uFE0F", "🎓"],
    ];
    for (const [input, want] of cases) expect(normalEmoji(input)).toBe(want);
  });

  it("already has the defaults in that form", () => {
    for (const e of COMMON_EMOJI) expect(normalEmoji(e)).toBe(e);
  });
});

describe("recent emoji", () => {
  it("puts the newest first, once, and keeps a few", () => {
    expect(addRecent(["😀", "🍔"], "🍔")).toEqual(["🍔", "😀"]);
    expect(addRecent(["😀"], "❤")).toEqual(["❤\uFE0F", "😀"]);
    expect(addRecent(Array.from({ length: 30 }, (_, i) => String(i)), "🍕")).toHaveLength(24);
  });

  it("reads only lists of strings", () => {
    expect(parseRecent('["😀","🍔"]')).toEqual(["😀", "🍔"]);
    expect(parseRecent('["😀",3]')).toEqual(["😀"]);
    expect(parseRecent("{")).toEqual([]);
    expect(parseRecent(null)).toEqual([]);
  });

  it("fills the quick reactions with the defaults", () => {
    expect(quickReactions(["🍕", "👍"], ["👍", "❤️", "😂"], 4)).toEqual(["🍕", "👍", "❤️", "😂"]);
    expect(quickReactions([], ["👍", "❤️"], 1)).toEqual(["👍"]);
  });
});

describe("gridMove", () => {
  // Two sections: 3 + 2 buttons in rows of 3 (the second starts a row).
  const at = [
    { x: 0, y: 0 }, { x: 10, y: 0 }, { x: 20, y: 0 },
    { x: 0, y: 30 }, { x: 10, y: 30 },
  ];
  it("moves along the list with left and right, and stays inside", () => {
    expect(gridMove(at, 0, "ArrowRight")).toBe(1);
    expect(gridMove(at, 2, "ArrowRight")).toBe(3);
    expect(gridMove(at, 0, "ArrowLeft")).toBe(0);
    expect(gridMove(at, 4, "ArrowRight")).toBe(4);
  });

  it("moves to the nearest button in the next or last row", () => {
    expect(gridMove(at, 1, "ArrowDown")).toBe(4);
    expect(gridMove(at, 2, "ArrowDown")).toBe(4);
    expect(gridMove(at, 4, "ArrowUp")).toBe(1);
    expect(gridMove(at, 4, "ArrowDown")).toBe(4);
  });

  it("goes back to the search field from the first row", () => {
    expect(gridMove(at, 1, "ArrowUp")).toBe(-1);
  });

  it("knows Home and End, and nothing else", () => {
    expect(gridMove(at, 3, "Home")).toBe(0);
    expect(gridMove(at, 1, "End")).toBe(4);
    expect(gridMove(at, 1, "a")).toBe(null);
  });
});
