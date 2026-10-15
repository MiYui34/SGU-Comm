import type { OutgoingAudio } from "@kallisti/protocol";
import { createEncoderRotation } from "./encoderRotation";
import { shouldFallBack } from "./fallback";
import { levelDb, shouldTransmit, vadStep, voiceLevel, type TransmitMode, type VadState } from "./vad";
import rnnoiseUrl from "@jitsi/rnnoise-wasm/dist/rnnoise.wasm?url";
import { fallbackMode, noiseLabel, type NoiseMode } from "./noise";

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
  // Noise suppression (see noise.ts): the browser's, RNNoise in our worklet,
  // or DeepFilterNet3 in a worklet before it. With either of ours the
  // browser's is off. running: the one that really runs. The echo test
  // plays our worklet's frames.
  let noise: NoiseMode = "browser";
  let running = $state<NoiseMode | null>(null);
  let noiseError = $state<string | null>(null);
  let rnnoise: Promise<ArrayBuffer> | null = null; // its bytes (see the worklet)
  let dfn: { node: AudioWorkletNode; dispose: () => void } | null = null;
  let dfnLoading = false;
  let device: string | null = null; // the one asked for last (see start)
  const echo = new GainNode(ctx, { gain: 0 });
  const ECHO_GAIN = 0.7; // a little quieter: less feedback on speakers
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
        else if (d.denoising !== undefined) running = d.denoising ? "strong" : running === "strong" ? null : running;
        else if (d.denoiseError) noiseFailed("strong", d.denoiseError);
      };
      void applyNoise();
      return node;
    })());

  // RNNoise's bytes, fetched once, when first used.
  const loadRnnoise = () =>
    (rnnoise ??= fetch(rnnoiseUrl).then((r) => (r.ok ? r.arrayBuffer() : Promise.reject(new Error(r.statusText)))));

  // Runs the chosen filter: RNNoise in our worklet, DeepFilterNet3 before
  // it (its code and model load when first chosen). A change while one
  // loads wins.
  const applyNoise = async () => {
    if (!node) return; // the node applies it when it is made
    if (noise !== "strongest" && dfn) {
      dfn.dispose();
      dfn = null;
      if (running === "strongest") running = null;
      route();
    }
    if (noise !== "strong") node.port.postMessage({ denoise: null });
    try {
      if (noise === "strong") {
        const bytes = await loadRnnoise();
        if (noise === "strong") node.port.postMessage({ denoise: bytes }); // copied: kept for next time
      } else if (noise === "strongest" && !dfn && !dfnLoading) {
        dfnLoading = true;
        try {
          const { createDeepFilterNetAudioWorklet } = await import("@workadventure/noise-suppression/deepfilternet");
          const w = await createDeepFilterNetAudioWorklet(ctx, {
            onOverload: () => noiseFailed("strongest", "this computer is too slow for it"),
          });
          if (noise !== "strongest") return w.dispose();
          dfn = w;
          w.node.connect(node);
          route();
          await w.ready;
          if (dfn === w) running = "strongest";
        } finally {
          dfnLoading = false;
        }
        if (noise !== "strongest") void applyNoise(); // changed while it loaded
      }
    } catch (err) {
      if (noise === "strong") rnnoise = null;
      noiseFailed(noise, err instanceof Error ? err.message : String(err));
    }
  };
  // A filter that failed (or is too slow): the next lighter one, with a
  // note. Not if the user chose another meanwhile. The browser's filter
  // needs the microphone opened again.
  const noiseFailed = (from: NoiseMode, why: string) => {
    if (noise !== from || from === "browser") return;
    noise = fallbackMode(from);
    noiseError = `${noiseLabel(from)} could not run (${why}). Now on: ${noiseLabel(noise)}.`;
    void applyNoise();
    if (noise === "browser" && stream) void start(device);
  };
  // The microphone goes through DeepFilterNet3 when it runs.
  const route = () => {
    if (!source || !node) return;
    source.disconnect();
    source.connect(dfn?.node ?? node);
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
          noiseSuppression: noise === "browser", // ours do it, not twice
          autoGainControl: true,
        },
      });
      // close() or another switch can run while the permission prompt is open.
      if (stale()) return next.getTracks().forEach((t) => t.stop());
      await workletNode();
      if (stale()) return next.getTracks().forEach((t) => t.stop());
      stopStream();
      stream = next;
      source = ctx.createMediaStreamSource(next);
      route();
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
    get running() {
      return running;
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
    // The microphone opens again when the browser's own filter goes on or
    // off: it is set when it opens.
    setNoiseSuppression: (m: NoiseMode) => {
      if (m === noise) return;
      const reopen = (m === "browser") !== (noise === "browser");
      noise = m;
      noiseError = null;
      void applyNoise();
      if (stream && reopen) void start(device);
    },
    // The echo test: what the microphone sends, after the filters.
    setEcho: (on: boolean) => {
      echo.gain.value = on ? ECHO_GAIN : 0;
    },
    close: () => {
      closed = true;
      stopStream();
      dfn?.dispose();
      // The node can still be on its way: disconnect it when it arrives.
      void nodePromise?.then((n) => n.disconnect()).catch(() => {});
      encoders.closeAll();
    },
  };
};
