import { describe, expect, it } from "vitest";
import { chunkOutcome, nextStart, startsAnew, type PlayoutConfig } from "../src/lib/voice/schedule";

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

describe("startsAnew", () => {
  it("is true for a new or stalled stream only", () => {
    expect(startsAnew(null, now)).toBe(true);
    expect(startsAnew(now - 10, now)).toBe(true);
    expect(startsAnew(now, now + 0.4)).toBe(false);
    expect(startsAnew(now + 4800, now)).toBe(false);
  });
});

// In µs: 20 ms chunks.
describe("chunkOutcome", () => {
  const chunk = (expected: number | null, timestamp: number, start: number | null, prevEnd: number | null) =>
    chunkOutcome({ expected, timestamp, duration: 20_000, start, prevEnd, now });

  it("is good when it goes on the stream in time", () => {
    expect(chunk(100_000, 100_000, now + 960, now + 960)).toEqual({ late: false, lost: 0 });
  });

  it("is good at the start of a talk spurt, after silence", () => {
    expect(chunk(null, 900_000, now + 2880, now - 5000)).toEqual({ late: false, lost: 0 });
  });

  it("is late when the stream ran dry before it came", () => {
    expect(chunk(100_000, 100_000, now + 2880, now - 10)).toEqual({ late: true, lost: 0 });
  });

  it("is late when it was dropped, or came after a newer one", () => {
    expect(chunk(100_000, 100_000, null, now + 14_401)).toEqual({ late: true, lost: 0 });
    expect(chunk(100_000, 60_000, now + 960, now + 960)).toEqual({ late: true, lost: 0 });
  });

  it("counts the chunks missing before it", () => {
    expect(chunk(100_000, 160_000, now + 2880, now - 10)).toEqual({ late: false, lost: 3 });
  });

  it("takes a long jump for a new talk spurt", () => {
    expect(chunk(100_000, 1_200_000, now + 2880, now - 10)).toEqual({ late: false, lost: 0 });
  });
});
