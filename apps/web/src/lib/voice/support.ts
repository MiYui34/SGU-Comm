// Browser features that voice needs (WebCodecs for Opus, a worklet for
// capture), by the names that are missing.
export const missingVoiceApis = (g: object = globalThis): string[] =>
  ["AudioEncoder", "AudioDecoder", "AudioWorkletNode"].filter((name) => typeof (g as Record<string, unknown>)[name] === "undefined");

export const voiceSupported = () => missingVoiceApis().length === 0;

export const unsupportedVoiceMessage = (missing: string[]): string | null =>
  missing.length
    ? `Voice does not work in this browser (it has no ${missing.join(", ")}). Use a current Chrome, Firefox or Safari.`
    : null;

// Choosing the output device: Chrome and Edge only.
export const outputDeviceSupported = () =>
  typeof AudioContext !== "undefined" && "setSinkId" in AudioContext.prototype;
