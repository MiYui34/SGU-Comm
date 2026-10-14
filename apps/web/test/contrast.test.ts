/// <reference types="node" />
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { contrast } from "../src/lib/contrast";

// The palette from app.css, so the test follows the real values.
const css = readFileSync(new URL("../src/app.css", import.meta.url), "utf8");
const color = (name: string) => {
  const m = new RegExp(`--${name}:\\s*(#[0-9a-f]{6})`, "i").exec(css);
  if (!m) throw new Error(`no --${name} in app.css`);
  return m[1]!;
};

describe("contrast", () => {
  it("follows the WCAG formula", () => {
    expect(contrast("#ffffff", "#000000")).toBeCloseTo(21, 1);
    expect(contrast("#777777", "#ffffff")).toBeCloseTo(4.48, 2);
  });
});

describe("palette (WCAG AA, 4.5:1 for text)", () => {
  const pairs: [string, string][] = [
    ["text", "bg-main"],
    ["text-muted", "bg-main"],
    ["text-muted", "bg-sidebar"],
    ["text-faint", "bg-main"],
    ["text-faint", "bg-input"],
    ["accent", "bg-input"], // user panel status
    ["accent", "bg-main"],
    ["link", "bg-main"],
    ["danger", "bg-input"],
  ];
  it.each(pairs)("%s on %s", (fg, bg) => {
    expect(contrast(color(fg), color(bg))).toBeGreaterThanOrEqual(4.5);
  });

  it("white on accent-strong (buttons, server icon, menu hover)", () => {
    expect(contrast("#ffffff", color("accent-strong"))).toBeGreaterThanOrEqual(4.5);
  });
});
