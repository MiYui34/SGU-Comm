import { describe, expect, it } from "vitest";
import { bufferLabel, playoutFor } from "../src/lib/voice/buffer";

// In samples at 48 kHz.
describe("playoutFor", () => {
  it("starts after the chosen buffer", () => {
    expect(playoutFor("low", false).startDelay).toBe(2880);
    expect(playoutFor("medium", false).startDelay).toBe(5760);
    expect(playoutFor("high", false).startDelay).toBe(12_000);
  });

  it("lets the queue grow 240 ms beyond the start delay", () => {
    expect(playoutFor("low", false)).toEqual({ startDelay: 2880, maxLatency: 14_400 });
    expect(playoutFor("high", false).maxLatency).toBe(12_000 + 11_520);
  });

  it("gives music 500 ms, or the setting if larger", () => {
    expect(playoutFor("low", true)).toEqual({ startDelay: 24_000, maxLatency: 35_520 });
    expect(playoutFor("high", true).startDelay).toBe(24_000);
  });
});

describe("bufferLabel", () => {
  it("names the level and its delay", () => {
    expect(bufferLabel("medium")).toBe("中（120 ms）");
  });
});
