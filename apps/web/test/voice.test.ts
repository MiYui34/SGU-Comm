import { describe, expect, it } from "vitest";
import { micWanted } from "../src/lib/voice";

describe("micWanted", () => {
  it("opens the microphone only in voice and unmuted, or for the settings", () => {
    expect(micWanted({ inVoice: true, selfMute: false, settingsOpen: false })).toBe(true);
    expect(micWanted({ inVoice: true, selfMute: true, settingsOpen: false })).toBe(false);
    expect(micWanted({ inVoice: false, selfMute: false, settingsOpen: false })).toBe(false);
    // The settings show the level and test the threshold.
    expect(micWanted({ inVoice: false, selfMute: true, settingsOpen: true })).toBe(true);
  });
});
