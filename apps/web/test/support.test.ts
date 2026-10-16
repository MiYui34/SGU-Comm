import { describe, expect, it } from "vitest";
import { missingVoiceApis, unsupportedVoiceMessage } from "../src/lib/voice/support";

describe("missingVoiceApis", () => {
  it("names the browser features voice needs that are missing", () => {
    expect(missingVoiceApis({ AudioDecoder: 1, AudioWorkletNode: 1 })).toEqual(["AudioEncoder"]);
    expect(missingVoiceApis({ AudioDecoder: 1, AudioEncoder: 1, AudioWorkletNode: 1 })).toEqual([]);
  });
  it("makes a message only when something is missing", () => {
    expect(unsupportedVoiceMessage([])).toBeNull();
    expect(unsupportedVoiceMessage(["AudioEncoder"])).toContain("AudioEncoder");
  });
});
