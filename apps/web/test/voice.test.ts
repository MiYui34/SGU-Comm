import { describe, expect, it } from "vitest";
import { micWanted } from "../src/lib/voice";

describe("micWanted", () => {
  const s = { inVoice: true, selfMute: false, settingsOpen: false, closeWhenMuted: false };
  it("opens the microphone in voice (also muted: no dropout on unmute), or for the settings", () => {
    expect(micWanted(s)).toBe(true);
    expect(micWanted({ ...s, selfMute: true })).toBe(true);
    expect(micWanted({ ...s, inVoice: false })).toBe(false);
    // The settings show the level and test the threshold.
    expect(micWanted({ ...s, inVoice: false, selfMute: true, settingsOpen: true })).toBe(true);
  });

  it("closes it when muted if the user wants that", () => {
    expect(micWanted({ ...s, closeWhenMuted: true })).toBe(true);
    expect(micWanted({ ...s, closeWhenMuted: true, selfMute: true })).toBe(false);
    expect(micWanted({ ...s, closeWhenMuted: true, selfMute: true, settingsOpen: true })).toBe(true);
  });
});
