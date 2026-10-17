import { describe, expect, it } from "vitest";
import { emojiParts } from "../src/lib/emoji";

describe("emojiParts", () => {
  it("finds text-style emoji with skin tones (parser 17.0.2 does not)", () => {
    expect(emojiParts("✌\uFE0F ✌🏻", "/e/")).toEqual([
      { emoji: "✌\uFE0F", url: "/e/270c.png" },
      { text: " " },
      { emoji: "✌🏻", url: "/e/270c-1f3fb.png" },
    ]);
  });

  it("splits text into text and emoji parts", () => {
    expect(emojiParts("a😀 👍🏽", "/e/")).toEqual([
      { text: "a" },
      { emoji: "😀", url: "/e/1f600.png" },
      { text: " " },
      { emoji: "👍🏽", url: "/e/1f44d-1f3fd.png" },
    ]);
    expect(emojiParts("❤️", "/e/")).toEqual([{ emoji: "❤️", url: "/e/2764.png" }]);
    expect(emojiParts("plain", "/e/")).toEqual([{ text: "plain" }]);
  });
  it("keeps a variation selector with its emoji (the parser gives it alone, without an image)", () => {
    expect(emojiParts("✅\uFE0F!", "/e/")).toEqual([{ emoji: "✅\uFE0F", url: "/e/2705.png" }, { text: "!" }]);
  });
});
