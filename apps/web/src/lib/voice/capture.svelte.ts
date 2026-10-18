import type { OutgoingAudio } from "@kallisti/protocol";
import { createEncoderRotation } from "./encoderRotation";
import { shouldFallBack } from "./fallback";
import { levelDb, shouldTransmit, vadStep, voiceLevel, type TransmitMode, type VadState } from "./vad";
import rnnoiseUrl from "@jitsi/rnnoise-wasm/dist/rnnoise.wasm?url";
import { fallbackMode, noiseLabel, type NoiseMode } from "./noise";
import { micConstraints } from "../settings";
import { encoderConfig, FRAME_MS, SAMPLE_RATE, sendChannels } from "./opus";

const FRAME_SIZE = 960; // 20 ms
// Mumble counts frames in 10 ms steps.
const MUMBLE_FRAME_US = 10_000;
const VAD_HOLD_MS = 300;

export type Capture = ReturnType<typeof createCapture>;

// One DeepFilterNet3 node per AudioContext, made when first chosen and
// kept: the package's dispose() does not free its state (10 to 14 MB each
// time), and the context outlives connections. Off, it is out of the path.
// handlers: the capture using it, for its overload and error messages.
type SharedDfn = {
  node: AudioWorkletNode;
  ready: Promise<unknown>;
  handlers: { overload: () => void; error: (why: string) => void } | null;
};
const dfns = new WeakMap<AudioContext, Promise<SharedDfn>>();
const sharedDfn = (ctx: AudioContext): Promise<SharedDfn> => {
  let p = dfns.get(ctx);
  if (!p) {
    p = (async () => {
      const { createDeepFilterNetAudioWorklet } = await import("@workadventure/noise-suppression/deepfilternet");
      const shared: SharedDfn = { node: null!, ready: Promise.resolve(), handlers: null };
      const w = await createDeepFilterNetAudioWorklet(ctx, { onOverload: () => shared.handlers?.overload() });
      shared.node = w.node;
      shared.ready = w.ready;
      // Errors while it runs (the package then passes the microphone through).
      w.node.port.addEventListener("message", (e: MessageEvent<{ type?: string; message?: string }>) => {
        if (e.data.type === "error") shared.handlers?.error(e.data.message ?? "it stopped");
      });
      return shared;
    })();
    p.catch(() => dfns.delete(ctx)); // a failed load may be tried again
    dfns.set(ctx, p);
  }
  return p;
};

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
  let cancelEcho = true; // see Settings.echoCancellation
  // See Settings.musicMode: no filter at all. channels: what the frames
  // have (see the worklet), so what the encoder gets.
  let music = false;
  let channels = 1;
  const filter = (): NoiseMode | null => (music ? null : noise);
  let running = $state<NoiseMode | null>(null);
  let noiseError = $state<string | null>(null);
  let rnnoise: Promise<ArrayBuffer> | null = null; // its bytes (see the worklet)
  let dfn: SharedDfn | null = null; // in use by this capture
  let device: string | null = null; // the one asked for last (see start)
  const echo = new GainNode(ctx, { gain: 0 });
  const ECHO_GAIN = 0.7; // a little quieter: less feedback on speakers
  // A fresh encoder for every talk spurt (see encoderRotation). madeFor:
  // the settings of the current one.
  const encoderKey = () => `${music}/${channels}`;
  let madeFor = "";
  const makeEncoder = (): AudioEncoder => {
    const encoder: AudioEncoder = new AudioEncoder({
      output: (chunk) => {
        const input = encoders.output(encoder);
        if (!input) return; // after the last input (see encoderRotation)
        const opus = new Uint8Array(chunk.byteLength);
        chunk.copyTo(opus);
        send({ target: 0, frameNumber: Math.round(input.timestamp / MUMBLE_FRAME_US), opus, terminator: input.terminator });
      },
      error: (err) => {
        console.warn("encoder error", err);
        micError = `Voice encoder failed: ${err.message}`;
      },
    });
    encoder.configure(encoderConfig(music, channels));
    madeFor = encoderKey();
    // close() throws on an encoder that already failed and closed itself.
    const close = encoder.close.bind(encoder);
    encoder.close = () => {
      if (encoder.state !== "closed") close();
    };
    return encoder;
  };
  const encoders = createEncoderRotation<AudioEncoder, { timestamp: number; terminator: boolean }>(makeEncoder);

  const encode = (samples: Float32Array<ArrayBuffer>, terminator: boolean) => {
    const encoder = encoders.current();
    if (encoder.state !== "configured") return;
    const data = new AudioData({
      format: "f32-planar",
      sampleRate: SAMPLE_RATE,
      numberOfFrames: FRAME_SIZE,
      numberOfChannels: channels,
      timestamp,
      data: samples,
    });
    encoders.encoded(encoder, { timestamp, terminator });
    timestamp += FRAME_MS * 1000;
    encoder.encode(data);
    data.close();
    // The last frame: flush, so an encoder that holds a packet back gives it
    // out. An extra packet after it (Chrome's lookahead) is dropped.
    if (terminator) encoder.flush().catch(() => {}); // closed meanwhile
  };

  const onFrame = (samples: Float32Array<ArrayBuffer>, voiceProbability: number | undefined) => {
    if (!source) return; // a frame still on its way after stop()
    // Music mode on or off, or mono and stereo: a new encoder. The old one
    // gives out the packet it holds back (see encode).
    channels = samples.length / FRAME_SIZE;
    if (encoderKey() !== madeFor) {
      const old = encoders.current();
      if (old.state === "configured") old.flush().catch(() => {});
      encoders.renew();
    }
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
  const workletNode = () => {
    if (nodePromise) return nodePromise;
    nodePromise = (async () => {
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
    })();
    nodePromise.catch(() => (nodePromise = null)); // a failed load may be tried again
    return nodePromise;
  };

  // RNNoise's bytes, fetched once, when first used.
  const loadRnnoise = () =>
    (rnnoise ??= fetch(rnnoiseUrl).then((r) => (r.ok ? r.arrayBuffer() : Promise.reject(new Error(r.statusText)))));

  // Runs the chosen filter: RNNoise in our worklet, DeepFilterNet3 before
  // it (the shared node, see sharedDfn). A change while one loads wins;
  // a failure counts for the filter that was loading.
  const applyNoise = async () => {
    if (!node) return; // the node applies it when it is made
    const want = filter();
    if (want !== "strongest" && dfn) {
      dfn.node.disconnect();
      dfn.handlers = null;
      dfn = null;
      if (running === "strongest") running = null;
      route();
    }
    if (want !== "strong") node.port.postMessage({ denoise: null });
    try {
      if (want === "strong") {
        const bytes = await loadRnnoise();
        if (filter() === "strong") node.port.postMessage({ denoise: bytes }); // copied: kept for next time
      } else if (want === "strongest" && !dfn) {
        const shared = await sharedDfn(ctx);
        if (closed || filter() !== "strongest" || dfn) return;
        shared.handlers = {
          overload: () => noiseFailed("strongest", "this computer is too slow for it"),
          error: (why) => noiseFailed("strongest", why),
        };
        shared.node.connect(node);
        dfn = shared;
        route();
        await shared.ready;
        if (dfn === shared) running = "strongest";
      }
    } catch (err) {
      if (want === "strong") rnnoise = null;
      if (want === "strongest") dfns.delete(ctx); // a node that did not start: made again next time
      if (want) noiseFailed(want, err instanceof Error ? err.message : String(err));
    }
  };
  // A filter that failed (or is too slow): the next lighter one, with a
  // note. Not if the user chose another meanwhile. The browser's filter
  // needs the microphone opened again.
  const noiseFailed = (from: NoiseMode, why: string) => {
    if (filter() !== from || from === "browser") return;
    noise = fallbackMode(from);
    noiseError = `${noiseLabel(from)} could not run (${why}). Now on: ${noiseLabel(noise)}.`;
    void applyNoise();
    if (noise === "browser" && stream) void start(device);
  };
  // The filter changed (was: the one before). The microphone opens again
  // when the browser's own filter goes on or off (or reopen): it is set
  // when it opens.
  const refilter = (was: NoiseMode | null, reopen = false) => {
    noiseError = null;
    void applyNoise();
    if (stream && (reopen || (was === "browser") !== (filter() === "browser"))) void start(device);
  };
  // The microphone goes through DeepFilterNet3 when it runs.
  const route = () => {
    if (!source || !node) return;
    source.disconnect();
    source.connect(dfn?.node ?? node);
  };

  const stopTracks = (s: MediaStream | null) => s?.getTracks().forEach((t) => t.stop());
  const stopStream = () => {
    source?.disconnect();
    stopTracks(stream);
    source = stream = null;
  };

  // Opens the microphone (null: the default device). Also used to switch.
  const start = async (deviceId: string | null = null) => {
    const mine = ++generation;
    device = deviceId;
    const stale = () => closed || mine !== generation;
    let next: MediaStream | null = null; // stopped on failure until it is stream
    try {
      // The browser's noise filter only without ours (not twice).
      const constraints = () => micConstraints(deviceId, { browserNoise: filter() === "browser", cancelEcho, music });
      const asked = constraints();
      next = await navigator.mediaDevices.getUserMedia({ audio: asked });
      // close() or another switch can run while the permission prompt is open.
      if (stale()) return stopTracks(next);
      // The filters changed meanwhile: ask again.
      if (JSON.stringify(constraints()) !== JSON.stringify(asked)) {
        stopTracks(next);
        return await start(deviceId);
      }
      const n = await workletNode();
      if (stale()) return stopTracks(next);
      stopStream();
      stream = next;
      const track = next.getAudioTracks()[0];
      // Stereo in music mode, if the microphone has two channels.
      const ch = sendChannels(music, track?.getSettings().channelCount);
      n.channelCount = ch;
      n.port.postMessage({ channels: ch });
      source = ctx.createMediaStreamSource(next);
      route();
      micError = null;
      deviceLabel = track?.label ?? "";
      fellBack = false;
      // Unplugged while in use: go on with the default microphone.
      if (track) track.onended = () => {
        if (!stale() && deviceId !== null) void start(null).then(() => (fellBack = !micError));
      };
    } catch (err) {
      if (next !== stream) stopTracks(next);
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
    setNoiseSuppression: (m: NoiseMode) => {
      if (m === noise) return;
      const was = filter();
      noise = m;
      refilter(was);
    },
    // Also set when the microphone opens: it opens again (music mode has
    // it off anyway).
    setEchoCancellation: (on: boolean) => {
      if (on === cancelEcho) return;
      cancelEcho = on;
      if (stream && !music) void start(device);
    },
    // The filters, the microphone's settings and (with the next frame) the
    // encoder all change.
    setMusicMode: (on: boolean) => {
      if (on === music) return;
      const was = filter();
      music = on;
      refilter(was, true);
    },
    // The echo test: what the microphone sends, after the filters.
    setEcho: (on: boolean) => {
      echo.gain.value = on ? ECHO_GAIN : 0;
    },
    close: () => {
      closed = true;
      stopStream();
      if (dfn) {
        dfn.node.disconnect(); // kept for the next capture (see sharedDfn)
        dfn.handlers = null;
      }
      // The node can still be on its way: disconnect it when it arrives.
      void nodePromise?.then((n) => n.disconnect()).catch(() => {});
      encoders.closeAll();
    },
  };
};
