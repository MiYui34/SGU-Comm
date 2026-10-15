import { describe, expect, it } from "vitest";
import { fallbackMode, noiseLabel } from "../src/lib/voice/noise";
import { parseSettings } from "../src/lib/settings";

describe("noise suppression modes", () => {
  it("falls back one step: DeepFilterNet3, RNNoise, the browser's filter", () => {
    expect(fallbackMode("strongest")).toBe("strong");
    expect(fallbackMode("strong")).toBe("browser");
    expect(fallbackMode("browser")).toBe("browser");
  });

  it("names the running filter", () => {
    expect(noiseLabel("strongest")).toBe("DeepFilterNet3");
    expect(noiseLabel("strong")).toBe("RNNoise");
  });

  it("keeps the setting", () => {
    expect(parseSettings(JSON.stringify({ noiseSuppression: "strongest" })).noiseSuppression).toBe("strongest");
    expect(parseSettings(JSON.stringify({ noiseSuppression: "loudest" })).noiseSuppression).toBe("browser");
  });
});
