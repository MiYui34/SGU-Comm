import { describe, expect, it } from "vitest";
import { nextStart, type PlayoutConfig } from "../src/lib/voice/schedule";

// In samples at 48 kHz: 60 ms start delay, 300 ms maximum queue.
const cfg: PlayoutConfig = { startDelay: 2880, maxLatency: 14400 };
const now = 480_000;

describe("nextStart", () => {
  it("starts a new stream after the start delay", () => {
    expect(nextStart(null, now, cfg)).toBe(now + 2880);
  });

  it("plays the next chunk right after the previous one", () => {
    expect(nextStart(now + 4800, now, cfg)).toBe(now + 4800);
  });

  it("restarts with the delay when the stream fell behind", () => {
    expect(nextStart(now - 10, now, cfg)).toBe(now + 2880);
  });

  it("drops the chunk when the queue is too long, so audio never overlaps", () => {
    expect(nextStart(now + 14401, now, cfg)).toBeNull();
  });

  it("rounds a fractional clock to whole samples", () => {
    expect(nextStart(null, now + 0.4, cfg)).toBe(now + 2880);
  });
});
