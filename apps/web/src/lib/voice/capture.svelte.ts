import type { OutgoingAudio } from "@mumble-discord/protocol";
import { levelDb, shouldTransmit, vadStep, type TransmitMode, type VadState } from "./vad";

const SAMPLE_RATE = 48_000;
const FRAME_SIZE = 960; // 20 ms
const FRAME_MS = 20;
// Mumble counts frames in 10 ms steps.
const MUMBLE_FRAME_US = 10_000;
const VAD_HOLD_MS = 300;

export type Capture = ReturnType<typeof createCapture>;

export const createCapture = (ctx: AudioContext, send: (packet: OutgoingAudio) => void) => {
  let level = $state(-100);
  let transmitting = $state(false);
  let micError = $state<string | null>(null);
  let muted = false;
  let mode: TransmitMode = "vad";
  let pttDown = false;
  let thresholdDb = -45;
  let vad: VadState = { talking: false, silentMs: 0 };
  let timestamp = 0;
  let closed = false;
  // Bumped on every device switch, so a slow older getUserMedia loses.
  let generation = 0;
  let stream: MediaStream | null = null;
  let source: MediaStreamAudioSourceNode | null = null;
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
    vad = vadStep(vad, level, FRAME_MS, { thresholdDb, holdMs: VAD_HOLD_MS });
    const next = shouldTransmit({ mode, vadTalking: vad.talking, pttDown, muted });
    // The first quiet frame after talking ends the transmission.
    if (next || transmitting) encode(samples, !next);
    transmitting = next;
  };

  // Made once. Shared promise, so two quick device switches cannot make two.
  let nodePromise: Promise<AudioWorkletNode> | null = null;
  const workletNode = () =>
    (nodePromise ??= (async () => {
      await ctx.audioWorklet.addModule(new URL("./capture.worklet.js", import.meta.url));
      node = new AudioWorkletNode(ctx, "capture", {
        numberOfOutputs: 0,
        channelCount: 1,
        channelCountMode: "explicit",
        processorOptions: { frameSize: FRAME_SIZE },
      });
      node.port.onmessage = (event: MessageEvent<Float32Array<ArrayBuffer>>) =>
        onFrame(event.data);
      return node;
    })());

  const stopStream = () => {
    source?.disconnect();
    stream?.getTracks().forEach((t) => t.stop());
    source = stream = null;
  };

  // Opens the microphone (null: the default device). Also used to switch.
  const start = async (deviceId: string | null = null) => {
    const mine = ++generation;
    const stale = () => closed || mine !== generation;
    try {
      const next = await navigator.mediaDevices.getUserMedia({
        audio: {
          deviceId: deviceId ? { exact: deviceId } : undefined,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      });
      // close() or another switch can run while the permission prompt is open.
      if (stale()) return next.getTracks().forEach((t) => t.stop());
      const target = await workletNode();
      if (stale()) return next.getTracks().forEach((t) => t.stop());
      stopStream();
      stream = next;
      source = ctx.createMediaStreamSource(next);
      source.connect(target);
      micError = null;
    } catch (err) {
      if (!stale()) micError = err instanceof Error ? err.message : String(err);
    }
  };

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
      // A key held during the switch never sends its keyup to us.
      pttDown = false;
    },
    setPttDown: (value: boolean) => {
      pttDown = value;
    },
    setThreshold: (db: number) => {
      thresholdDb = db;
    },
    close: () => {
      closed = true;
      stopStream();
      // The node can still be on its way: disconnect it when it arrives.
      void nodePromise?.then((n) => n.disconnect()).catch(() => {});
      if (encoder.state !== "closed") encoder.close();
    },
  };
};
