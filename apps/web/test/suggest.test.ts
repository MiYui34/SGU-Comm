import { describe, expect, it } from "vitest";
import { listKey, quoted, replaceTyped, typedFilter, typedWord } from "../src/lib/suggest";

describe("typedWord (message box)", () => {
  it("finds the @word before the caret", () => {
    expect(typedWord("hi @la", 6)).toEqual({ start: 3, kind: "@", query: "la" });
    expect(typedWord("@", 1)).toEqual({ start: 0, kind: "@", query: "" });
    expect(typedWord("(@b", 3)).toEqual({ start: 1, kind: "@", query: "b" });
    expect(typedWord("mail@la", 7)).toBeNull();
    expect(typedWord("hi @la x", 8)).toBeNull(); // the word ended
    expect(typedWord("hi @la x", 6)).toEqual({ start: 3, kind: "@", query: "la" });
    expect(typedWord("cc:@bo", 6)).toEqual({ start: 3, kind: "@", query: "bo" }); // as mentions are found
  });

  it("finds #channel and :emoji (two letters at least, as Discord)", () => {
    expect(typedWord("see #gen", 8)).toEqual({ start: 4, kind: "#", query: "gen" });
    expect(typedWord("issue#12", 8)).toBeNull();
    expect(typedWord("nice :sm", 8)).toEqual({ start: 5, kind: ":", query: "sm" });
    expect(typedWord("nice :s", 7)).toBeNull();
    expect(typedWord("at 10:30", 8)).toBeNull();
    expect(typedWord("https://x", 9)).toBeNull();
  });
});

describe("typedFilter (search box)", () => {
  it("finds from: and in: before the caret", () => {
    expect(typedFilter("pizza from:bo", 13)).toEqual({ start: 6, kind: "from", query: "bo" });
    expect(typedFilter("in:#ge", 6)).toEqual({ start: 0, kind: "in", query: "ge" });
    expect(typedFilter('in:"Lou', 7)).toEqual({ start: 0, kind: "in", query: "Lou" });
    expect(typedFilter('in:@"Bob S', 10)).toEqual({ start: 0, kind: "in", query: "Bob S" });
    expect(typedFilter("pizza", 5)).toBeNull();
    expect(typedFilter("xfrom:bo", 8)).toBeNull();
  });
});

describe("replaceTyped", () => {
  it("puts the choice and a space in place of the typed word, caret after it", () => {
    expect(replaceTyped("hi @la there", 3, 6, "@lain")).toEqual({ text: "hi @lain there", caret: 9 }); // after the space
    expect(replaceTyped("@", 0, 1, "@lain soy")).toEqual({ text: "@lain soy ", caret: 10 });
    expect(replaceTyped("nice :sm", 5, 8, "😄")).toEqual({ text: "nice 😄 ", caret: 8 });
  });

  it("quotes names with spaces for the search", () => {
    expect(quoted("Lounge 2")).toBe('"Lounge 2"');
    expect(quoted("bob")).toBe("bob");
  });
});

describe("listKey", () => {
  it("moves around the list, picks with Enter or Tab, closes with Escape", () => {
    expect(listKey("ArrowDown", 3, 2)).toEqual({ active: 0 });
    expect(listKey("ArrowUp", 3, 0)).toEqual({ active: 2 });
    expect(listKey("Enter", 3, 1)).toBe("pick");
    expect(listKey("Tab", 3, 1)).toBe("pick");
    expect(listKey("Escape", 3, 1)).toBe("close");
    expect(listKey("a", 3, 1)).toBeNull();
  });
});
