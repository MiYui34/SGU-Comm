import { describe, expect, it } from "vitest";
import { nextPanel, swipeDirection, togglePins } from "../src/lib/mobile";

const at = (x: number, y: number, t: number) => ({ x, y, t });

describe("swipeDirection", () => {
  it("detects quick horizontal swipes", () => {
    expect(swipeDirection(at(100, 300, 0), at(220, 310, 200))).toBe("right");
    expect(swipeDirection(at(300, 300, 0), at(150, 290, 200))).toBe("left");
  });

  it("ignores short, slow and vertical movements", () => {
    expect(swipeDirection(at(100, 300, 0), at(130, 300, 100))).toBeNull(); // short
    expect(swipeDirection(at(100, 300, 0), at(250, 300, 2000))).toBeNull(); // slow
    expect(swipeDirection(at(100, 300, 0), at(180, 450, 200))).toBeNull(); // mostly vertical (scrolling)
  });
});

describe("nextPanel", () => {
  it("opens the side panels from the chat", () => {
    expect(nextPanel("chat", "right")).toBe("channels");
    expect(nextPanel("chat", "left")).toBe("members");
  });

  it("goes back to the chat", () => {
    expect(nextPanel("channels", "left")).toBe("chat");
    expect(nextPanel("members", "right")).toBe("chat");
  });

  it("stays when the swipe leads nowhere", () => {
    expect(nextPanel("channels", "right")).toBe("channels");
    expect(nextPanel("members", "left")).toBe("members");
    expect(nextPanel("chat", null)).toBe("chat");
  });
});

describe("togglePins", () => {
  it("shows the pins, and hides them when they are on screen", () => {
    expect(togglePins("members", true)).toBe("pins");
    expect(togglePins("search", true)).toBe("pins");
    expect(togglePins("pins", true)).toBe("members");
  });

  it("shows pins kept in a closed phone drawer", () => {
    expect(togglePins("pins", false)).toBe("pins");
  });
});
