/// <reference types="node" />
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { contrast } from "../src/lib/contrast";

// The palettes from app.css, so the test follows the real values: dark in
// :root, light in its own block.
const css = readFileSync(new URL("../src/app.css", import.meta.url), "utf8");
const light = css.slice(css.indexOf('[data-theme="light"]'));
const palette = (block: string) => (name: string) => {
  const m = new RegExp(`--${name}:\\s*(#[0-9a-f]{6})`, "i").exec(block);
  if (!m) throw new Error(`no --${name} in app.css`);
  return m[1]!;
};

describe("contrast", () => {
  it("follows the WCAG formula", () => {
    expect(contrast("#ffffff", "#000000")).toBeCloseTo(21, 1);
    expect(contrast("#777777", "#ffffff")).toBeCloseTo(4.48, 2);
  });
});

describe.each([
  ["dark", palette(css)],
  ["light", palette(light)],
])("%s palette (WCAG AA, 4.5:1 for text)", (_, color) => {
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
    ["text", "bg-menu"],
    ["text-muted", "bg-rail"],
    ["text-strong", "bg-active"], // the open channel
  ];
  it.each(pairs)("%s on %s", (fg, bg) => {
    expect(contrast(color(fg), color(bg))).toBeGreaterThanOrEqual(4.5);
  });

  it("white on accent-strong (buttons, server icon, menu hover)", () => {
    expect(contrast("#ffffff", color("accent-strong"))).toBeGreaterThanOrEqual(4.5);
  });
});
