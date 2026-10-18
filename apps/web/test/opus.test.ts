import { describe, expect, it } from "vitest";
import { encoderConfig, sendChannels } from "../src/lib/voice/opus";

describe("encoderConfig", () => {
  it("sends speech at 40 kbps mono in 20 ms frames", () => {
    expect(encoderConfig(false, 1)).toEqual({
      codec: "opus",
      sampleRate: 48_000,
      numberOfChannels: 1,
      bitrate: 40_000,
      opus: { frameDuration: 20_000 },
    });
  });

  it("sends music at 128 kbps with Opus tuned for music, in the same frames", () => {
    expect(encoderConfig(true, 2)).toEqual({
      codec: "opus",
      sampleRate: 48_000,
      numberOfChannels: 2,
      bitrate: 128_000,
      opus: { frameDuration: 20_000, application: "audio", signal: "music", complexity: 10 },
    });
    expect(encoderConfig(true, 1).numberOfChannels).toBe(1);
  });
});

describe("sendChannels", () => {
  it("is stereo only in music mode with a two-channel microphone", () => {
    expect(sendChannels(true, 2)).toBe(2);
    expect(sendChannels(true, 6)).toBe(2);
    expect(sendChannels(true, 1)).toBe(1);
    expect(sendChannels(true, undefined)).toBe(1); // the browser does not say
    expect(sendChannels(false, 2)).toBe(1);
  });
});
