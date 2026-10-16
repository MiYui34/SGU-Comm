import { describe, expect, it } from "vitest";
import { nextScrollTop } from "../src/lib/scroll";

// A 500 px high view.
const before = { scrollHeight: 2000, scrollTop: 1500, clientHeight: 500 };

describe("nextScrollTop", () => {
  it("follows new messages when the view was at the bottom", () => {
    expect(nextScrollTop(before, 2100, false)).toBe(1600);
  });

  it("counts as at the bottom within a small margin", () => {
    expect(nextScrollTop({ ...before, scrollTop: 1450 }, 2100, false)).toBe(1600);
  });

  it("keeps the reading position when scrolled up", () => {
    // New message at the bottom: the content above does not move.
    expect(nextScrollTop({ ...before, scrollTop: 300 }, 2100, false)).toBe(300);
  });

  it("keeps the reading position when older messages load above", () => {
    expect(nextScrollTop({ ...before, scrollTop: 0 }, 2800, false, true)).toBe(800);
  });

  it("goes to the bottom on a channel switch", () => {
    expect(nextScrollTop({ ...before, scrollTop: 0 }, 900, true)).toBe(400);
  });
});

describe("nextScrollTop without a snapshot", () => {
  it("leaves the reader where they are (a re-run without new content), unless the conversation changed", () => {
    expect(nextScrollTop(null, 2654, false)).toBeNull();
    expect(nextScrollTop(null, 2654, true)).toBe(2654);
  });
});
