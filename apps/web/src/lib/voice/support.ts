export const voiceSupported = () =>
  typeof AudioDecoder !== "undefined" &&
  typeof AudioEncoder !== "undefined" &&
  typeof AudioWorkletNode !== "undefined";
