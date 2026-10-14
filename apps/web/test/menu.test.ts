import { describe, expect, it } from "vitest";
import { fitMenu } from "../src/lib/menu";

const view = { width: 1000, height: 800 };
const menu = { width: 200, height: 300 };

describe("fitMenu", () => {
  it("opens at the pointer when there is room", () => {
    expect(fitMenu({ x: 100, y: 100 }, menu, view)).toEqual({ x: 100, y: 100 });
  });

  it("opens to the left near the right edge", () => {
    expect(fitMenu({ x: 900, y: 100 }, menu, view)).toEqual({ x: 700, y: 100 });
  });

  it("opens upwards near the bottom edge", () => {
    expect(fitMenu({ x: 100, y: 700 }, menu, view)).toEqual({ x: 100, y: 400 });
  });

  it("stays inside a small window", () => {
    expect(fitMenu({ x: 50, y: 50 }, menu, { width: 150, height: 200 })).toEqual({
      x: 8,
      y: 8,
    });
  });
});
