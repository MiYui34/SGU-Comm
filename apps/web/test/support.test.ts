import { describe, expect, it } from "vitest";
import { missingVoiceApis, unsupportedVoiceMessage } from "../src/lib/voice/support";

describe("missingVoiceApis", () => {
  it("names the browser features voice needs that are missing", () => {
    expect(missingVoiceApis({ AudioWorkletNode: 1 })).toEqual(["WebAssembly"]);
    expect(missingVoiceApis({ AudioWorkletNode: 1, WebAssembly: 1 })).toEqual([]);
  });
  it("does not need WebCodecs: Opus in WebAssembly stands in", () => {
    expect(missingVoiceApis({ WebAssembly: 1 })).toEqual(["AudioWorkletNode"]);
  });
  it("makes a message only when something is missing", () => {
    expect(unsupportedVoiceMessage([])).toBeNull();
    expect(unsupportedVoiceMessage(["AudioWorkletNode"])).toContain("AudioWorkletNode");
  });
});
