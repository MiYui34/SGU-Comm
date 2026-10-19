import { describe, expect, it } from "vitest";
import { fullScreenStep, gridColumns, nextFocus, stripOf } from "../src/lib/stream/layout";

describe("gridColumns", () => {
  it("is one column for one tile or none", () => {
    expect(gridColumns(1, 1200, 600)).toBe(1);
    expect(gridColumns(0, 1200, 600)).toBe(1);
  });
  it("puts two tiles side by side in a wide area", () => {
    expect(gridColumns(2, 1600, 500)).toBe(2);
  });
  it("stacks them in a narrow, tall area (phones)", () => {
    expect(gridColumns(2, 360, 600)).toBe(1);
    expect(gridColumns(4, 360, 700)).toBe(1);
  });
  it("makes a square-ish grid in between", () => {
    expect(gridColumns(4, 1200, 700)).toBe(2);
    expect(gridColumns(9, 1600, 900)).toBe(3);
  });
  it("fills a square area by rows, a flat one in one row", () => {
    expect(gridColumns(3, 1000, 1000)).toBe(1);
    expect(gridColumns(3, 3000, 300)).toBe(3);
  });
  it("is one column before the area is measured", () => {
    expect(gridColumns(3, 0, 0)).toBe(1);
  });
});

describe("nextFocus", () => {
  it("focuses a stream just watched", () => {
    expect(nextFocus(1, [1], [1, 2])).toBe(2);
  });
  it("keeps the focus while its stream is there", () => {
    expect(nextFocus(2, [1, 2, 3], [2, 3])).toBe(2);
  });
  it("moves to the first one left when the focused stream ends", () => {
    expect(nextFocus(2, [1, 2, 3], [1, 3])).toBe(1);
  });
  it("is none when nothing is watched", () => {
    expect(nextFocus(2, [2], [])).toBeNull();
    expect(nextFocus(null, [], [])).toBeNull();
  });
  it("takes the first one when nothing was focused", () => {
    expect(nextFocus(null, [4, 5], [4, 5])).toBe(4);
  });
});

describe("stripOf", () => {
  it("is the others, in order", () => {
    expect(stripOf([3, 1, 2], 1)).toEqual([3, 2]);
    expect(stripOf([3, 1], null)).toEqual([3, 1]);
  });
});

describe("fullScreenStep", () => {
  it("puts the element in full screen, also from the full screen stage", () => {
    expect(fullScreenStep(null, "tile")).toBe("enter");
    expect(fullScreenStep("stage", "tile")).toBe("enter");
  });
  it("leaves full screen when the element is in it", () => {
    expect(fullScreenStep("tile", "tile")).toBe("exit");
  });
});
