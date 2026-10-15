import type { OutgoingAudio } from "@kallisti/protocol";
import { createEncoderRotation } from "./encoderRotation";
import { shouldFallBack } from "./fallback";
import { levelDb, shouldTransmit, vadStep, voiceLevel, type TransmitMode, type VadState } from "./vad";
import rnnoiseUrl from "@jitsi/rnnoise-wasm/dist/rnnoise.wasm?url";

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
  // The microphone really in use, and whether it replaced the chosen one.
  let deviceLabel = $state("");
  let fellBack = $state(false);
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
  // Noise suppression: the browser's, or RNNoise in the worklet (then the
  // browser's is off). The echo test plays the worklet's frames.
  let strong = false;
  let rnnoise: Promise<ArrayBuffer> | null = null; // its bytes (see the worklet)
  let noiseError = $state<string | null>(null);
  let denoising = $state(false); // RNNoise runs (the worklet said so)
  let device: string | null = null; // the one asked for last (see start)
  const echo = new GainNode(ctx, { gain: 0 });
  // Timestamps of frames that end a transmission. Keyed by timestamp, so a
  // dropped or merged frame cannot shift the flag to another packet.
  const terminators = new Set<number>();

  // A fresh encoder for every talk spurt (see encoderRotation).
  const makeEncoder = (): AudioEncoder => {
    const encoder: AudioEncoder = new AudioEncoder({
      output: (chunk) => {
        const opus = new Uint8Array(chunk.byteLength);
        chunk.copyTo(opus);
        const terminator = terminators.delete(chunk.timestamp);
        const frameNumber = Math.round(chunk.timestamp / MUMBLE_FRAME_US);
        send({ target: 0, frameNumber, opus, terminator });
        encoders.output(encoder);
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
    // close() throws on an encoder that already failed and closed itself.
    const close = encoder.close.bind(encoder);
    encoder.close = () => {
      if (encoder.state !== "closed") close();
    };
    return encoder;
  };
  const encoders = createEncoderRotation(makeEncoder);

  const encode = (samples: Float32Array<ArrayBuffer>, terminator: boolean) => {
    const encoder = encoders.current();
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
    encoders.encoded(encoder);
    encoder.encode(data);
    data.close();
    // No flush() after the last frame: it emits an extra packet for the
    // lookahead samples, after the terminator and with a repeated frame
    // number. Chrome gives one output per input, so nothing is held back.
  };

  const onFrame = (samples: Float32Array<ArrayBuffer>, voiceProbability: number | undefined) => {
    if (!source) return; // a frame still on its way after stop()
    level = levelDb(samples);
    vad = vadStep(vad, voiceLevel(level, voiceProbability), FRAME_MS, { thresholdDb, holdMs: VAD_HOLD_MS });
    const next = shouldTransmit({ mode, vadTalking: vad.talking, pttDown, muted });
    if (next && !transmitting) encoders.startSpurt();
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
        numberOfOutputs: 1,
        outputChannelCount: [1],
        channelCount: 1,
        channelCountMode: "explicit",
        processorOptions: { frameSize: FRAME_SIZE },
      });
      // Some browsers (Safari) only run nodes that lead to the speakers. The
      // output is the echo test's: silent (gain 0) unless it is on.
      node.connect(echo).connect(ctx.destination);
      node.port.onmessage = (
        event: MessageEvent<{ samples?: Float32Array<ArrayBuffer>; vad?: number; denoising?: boolean; denoiseError?: string }>,
      ) => {
        const d = event.data;
        if (d.samples) onFrame(d.samples, d.vad);
        else if (d.denoising !== undefined) denoising = d.denoising;
        else if (d.denoiseError) denoiseFailed(d.denoiseError);
      };
      void applyDenoise();
      return node;
    })());

  // Tells the worklet to run RNNoise or not (the module loads once, when
  // first used). A change while it loads wins.
  const applyDenoise = async () => {
    if (!node) return; // the node applies it when it is made
    try {
      const bytes = strong ? await (rnnoise ??= fetch(rnnoiseUrl).then((r) => (r.ok ? r.arrayBuffer() : Promise.reject(new Error(r.statusText))))) : null;
      if (strong === (bytes !== null)) node.port.postMessage({ denoise: bytes }); // copied: kept for next time
    } catch (err) {
      rnnoise = null;
      denoiseFailed(err instanceof Error ? err.message : String(err));
    }
  };
  // Back to the browser's filter (it was off for RNNoise): reopen.
  const denoiseFailed = (why: string) => {
    strong = false;
    noiseError = `Strong noise suppression could not start (${why}). The standard one is on.`;
    if (stream) void start(device);
  };

  const stopStream = () => {
    source?.disconnect();
    stream?.getTracks().forEach((t) => t.stop());
    source = stream = null;
  };

  // Opens the microphone (null: the default device). Also used to switch.
  const start = async (deviceId: string | null = null) => {
    const mine = ++generation;
    device = deviceId;
    const stale = () => closed || mine !== generation;
    try {
      const next = await navigator.mediaDevices.getUserMedia({
        audio: {
          deviceId: deviceId ? { exact: deviceId } : undefined,
          echoCancellation: true,
          noiseSuppression: !strong, // RNNoise does it, not twice
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
      const track = next.getAudioTracks()[0];
      deviceLabel = track?.label ?? "";
      fellBack = false;
      // Unplugged while in use: go on with the default microphone.
      if (track) track.onended = () => {
        if (!stale() && deviceId !== null) void start(null).then(() => (fellBack = !micError));
      };
    } catch (err) {
      if (stale()) return;
      if (shouldFallBack(err, deviceId)) {
        await start(null);
        fellBack = !micError;
        return;
      }
      micError = err instanceof Error ? err.message : String(err);
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
    get deviceLabel() {
      return deviceLabel;
    },
    get fellBack() {
      return fellBack;
    },
    get noiseError() {
      return noiseError;
    },
    get denoising() {
      return denoising;
    },
    start,
    // Closes the microphone (the browser stops showing it as recording);
    // start opens it again.
    stop: () => {
      generation++; // a getUserMedia still on its way is not used
      stopStream();
      level = -100;
      transmitting = false;
      vad = { talking: false, silentMs: 0 };
    },
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
    // Strong: RNNoise. Takes effect for the filter at once; the browser's
    // own filter changes when the microphone opens again (start).
    setStrongNoiseSuppression: (on: boolean) => {
      strong = on;
      noiseError = null;
      void applyDenoise();
    },
    // The echo test: what the microphone sends, after the filters.
    setEcho: (on: boolean) => {
      echo.gain.value = on ? 1 : 0;
    },
    close: () => {
      closed = true;
      stopStream();
      // The node can still be on its way: disconnect it when it arrives.
      void nodePromise?.then((n) => n.disconnect()).catch(() => {});
      encoders.closeAll();
    },
  };
};
