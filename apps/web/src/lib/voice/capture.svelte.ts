import type { OutgoingAudio } from "@mumble-discord/protocol";
import { levelDb, shouldTransmit, vadStep, type TransmitMode, type VadState } from "./vad";

const SAMPLE_RATE = 48_000;
const FRAME_SIZE = 960; // 20 ms
const FRAME_MS = 20;
// Mumble counts frames in 10 ms steps.
const MUMBLE_FRAME_US = 10_000;
const VAD = { thresholdDb: -45, holdMs: 300 };

export type Capture = ReturnType<typeof createCapture>;

export const createCapture = (ctx: AudioContext, send: (packet: OutgoingAudio) => void) => {
  let level = $state(-100);
  let transmitting = $state(false);
  let micError = $state<string | null>(null);
  let muted = false;
  let mode: TransmitMode = "vad";
  let pttDown = false;
  let vad: VadState = { talking: false, silentMs: 0 };
  let timestamp = 0;
  let closed = false;
  let stream: MediaStream | null = null;
  let node: AudioWorkletNode | null = null;
  // Timestamps of frames that end a transmission. Keyed by timestamp, so a
  // dropped or merged frame cannot shift the flag to another packet.
  const terminators = new Set<number>();

  const encoder = new AudioEncoder({
    output: (chunk) => {
      const opus = new Uint8Array(chunk.byteLength);
      chunk.copyTo(opus);
      const terminator = terminators.delete(chunk.timestamp);
      const frameNumber = Math.round(chunk.timestamp / MUMBLE_FRAME_US);
      send({ target: 0, frameNumber, opus, terminator });
    },
    error: (err) => {
      console.warn("encoder error", err);
      micError = `Voice encoder failed: ${err.message}`;
    },
  });
  encoder.configure({
    codec: "opus",
    sampleRate: SAMPLE_RATE,
    numberOfChannels: 1,
    bitrate: 40_000,
    opus: { frameDuration: FRAME_MS * 1000 },
  });

  const encode = (samples: Float32Array<ArrayBuffer>, terminator: boolean) => {
    if (encoder.state !== "configured") return;
    if (terminator) terminators.add(timestamp);
    const data = new AudioData({
      format: "f32-planar",
      sampleRate: SAMPLE_RATE,
      numberOfFrames: FRAME_SIZE,
      numberOfChannels: 1,
      timestamp,
      data: samples,
    });
    timestamp += FRAME_MS * 1000;
    encoder.encode(data);
    data.close();
    // No flush() after the last frame: it emits an extra packet for the
    // lookahead samples, after the terminator and with a repeated frame
    // number. Chrome gives one output per input, so nothing is held back.
  };

  const onFrame = (samples: Float32Array<ArrayBuffer>) => {
    level = levelDb(samples);
    vad = vadStep(vad, level, FRAME_MS, VAD);
    const next = shouldTransmit({ mode, vadTalking: vad.talking, pttDown, muted });
    // The first quiet frame after talking ends the transmission.
    if (next || transmitting) encode(samples, !next);
    transmitting = next;
  };

  const start = async () => {
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      });
      // close() can run while the permission prompt is open.
      if (closed) return stopStream();
      await ctx.audioWorklet.addModule(new URL("./capture.worklet.js", import.meta.url));
      if (closed) return stopStream();
      node = new AudioWorkletNode(ctx, "capture", {
        numberOfOutputs: 0,
        channelCount: 1,
        channelCountMode: "explicit",
        processorOptions: { frameSize: FRAME_SIZE },
      });
      node.port.onmessage = (event: MessageEvent<Float32Array<ArrayBuffer>>) => onFrame(event.data);
      ctx.createMediaStreamSource(stream).connect(node);
    } catch (err) {
      if (!closed) micError = err instanceof Error ? err.message : String(err);
    }
  };

  const stopStream = () => stream?.getTracks().forEach((t) => t.stop());

  return {
    get level() {
      return level;
    },
    get transmitting() {
      return transmitting;
    },
    get micError() {
      return micError;
    },
    start,
    setMuted: (value: boolean) => {
      muted = value;
    },
    setMode: (value: TransmitMode) => {
      mode = value;
    },
    setPttDown: (value: boolean) => {
      pttDown = value;
    },
    close: () => {
      closed = true;
      stopStream();
      node?.disconnect();
      if (encoder.state !== "closed") encoder.close();
    },
  };
};
