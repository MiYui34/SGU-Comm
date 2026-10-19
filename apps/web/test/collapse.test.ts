import { describe, expect, it } from "vitest";
import { collapses, textLines } from "../src/lib/collapse";

describe("textLines", () => {
  it("counts the lines of the text", () => {
    expect(textLines("one")).toBe(1);
    expect(textLines("one\ntwo\n\nfour")).toBe(4);
  });

  it("counts a long line by the characters that fit on a line", () => {
    expect(textLines("x".repeat(120))).toBe(1);
    expect(textLines("x".repeat(121))).toBe(2);
    expect(textLines("x".repeat(50), 20)).toBe(3);
  });

  it("counts the lines of a code block with its fences", () => {
    expect(textLines("look:\n```js\na()\nb()\n```")).toBe(5);
  });
});

describe("collapses", () => {
  it("keeps short messages open", () => {
    expect(collapses("hi")).toBe(false);
    expect(collapses(Array(20).fill("line").join("\n"))).toBe(false);
  });

  it("collapses more than 20 lines", () => {
    expect(collapses(Array(21).fill("line").join("\n"))).toBe(true);
  });

  it("collapses long wrapped text", () => {
    expect(collapses("x".repeat(20 * 120 + 1))).toBe(true);
    expect(collapses("x".repeat(2000), 20, 80)).toBe(true);
    expect(collapses("x".repeat(2000))).toBe(false);
  });

  it("collapses a long code block", () => {
    expect(collapses("```\n" + Array(30).fill("x = 1").join("\n") + "\n```")).toBe(true);
  });
});
