export const voiceSupported = () =>
  typeof AudioDecoder !== "undefined" &&
  typeof AudioEncoder !== "undefined" &&
  typeof AudioWorkletNode !== "undefined";

// Choosing the output device: Chrome and Edge only.
export const outputDeviceSupported = () =>
  typeof AudioContext !== "undefined" && "setSinkId" in AudioContext.prototype;
