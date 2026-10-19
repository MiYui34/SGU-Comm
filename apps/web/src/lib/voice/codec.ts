// Opus for capture and playback: WebCodecs where it has Opus, else libopus
// in WebAssembly (libopus-wasm, loaded only then). WebKitGTK (the Linux
// desktop app) does WebCodecs with GStreamer plugins, which may be
// missing. Both sides get the same small interface: planar float samples
// in, packets out, and back.
import type { Closable } from "./encoderRotation";
import { encoderConfig, SAMPLE_RATE } from "./opus";

export type CodecKind = "webcodecs" | "wasm";
type EncoderConfig = ReturnType<typeof encoderConfig>;
type Sink<Out extends unknown[]> = { output(...out: Out): void; error(err: Error): void };
export type EncoderSink = Sink<[opus: Uint8Array]>;
export type DecoderSink = Sink<[planes: Float32Array<ArrayBuffer>[], timestamp: number]>;
// samples: planar, numberOfChannels from the config. Outputs come in
// input order; flush() gives out a packet the encoder holds back.
export type VoiceEncoder = Closable & { encode(samples: Float32Array<ArrayBuffer>, timestamp: number): void; flush(): void };
// Two channels out: mono packets come out the same on both.
export type VoiceDecoder = Closable & { decode(opus: Uint8Array, timestamp: number): void };

const DECODER_CONFIG: AudioDecoderConfig = { codec: "opus", sampleRate: SAMPLE_RATE, numberOfChannels: 2 };

export const chooseCodec = async (g: object = globalThis): Promise<CodecKind> => {
  const { AudioEncoder: E, AudioDecoder: D } = g as Partial<Pick<typeof globalThis, "AudioEncoder" | "AudioDecoder">>;
  if (!E || !D) return "wasm";
  try {
    const checks = await Promise.all([
      E.isConfigSupported(encoderConfig(false, 1)),
      E.isConfigSupported(encoderConfig(true, 2)),
      D.isConfigSupported(DECODER_CONFIG),
    ]);
    return checks.every((c) => c.supported) ? "webcodecs" : "wasm";
  } catch {
    return "wasm";
  }
};

// libopus wants interleaved samples; WebAudio and our worklet are planar.
export const interleave = (planar: Float32Array<ArrayBuffer>, channels: number): Float32Array<ArrayBuffer> => {
  if (channels === 1) return planar;
  const frames = planar.length / channels;
  const out = new Float32Array(planar.length);
  for (let ch = 0; ch < channels; ch++) for (let i = 0; i < frames; i++) out[i * channels + ch] = planar[ch * frames + i]!;
  return out;
};

export const deinterleave = (mixed: Float32Array<ArrayBuffer>, channels: number): Float32Array<ArrayBuffer>[] => {
  if (channels === 1) return [mixed];
  const frames = mixed.length / channels;
  return Array.from({ length: channels }, (_, ch) => Float32Array.from({ length: frames }, (_, i) => mixed[i * channels + ch]!));
};

// libopus's constants (opus_defines.h). WebCodecs' default application is
// "audio".
const APPLICATION = { voip: 2048, audio: 2049, lowdelay: 2051 } as const;
const SIGNAL = { auto: -1000, voice: 3001, music: 3002 } as const;

export const libopusOptions = (c: EncoderConfig) => ({
  channels: c.numberOfChannels as 1 | 2,
  sampleRate: SAMPLE_RATE as 48_000,
  frameSize: ((c.opus.frameDuration ?? 20_000) * SAMPLE_RATE) / 1e6,
  bitrate: c.bitrate,
  application: APPLICATION[c.opus.application ?? "audio"],
  signal: SIGNAL[c.opus.signal ?? "auto"],
  complexity: c.opus.complexity,
});

const asError = (err: unknown) => (err instanceof Error ? err : new Error(String(err)));

// A codec that is still loading: calls wait for it and run in order. A
// failure (the load or a call) goes to onError; close() also closes one
// that arrives later.
export const whenReady = <T extends Closable>(loading: Promise<T>, onError: (err: Error) => void) => {
  let ready: T | null = null;
  let closed = false;
  let queue: ((t: T) => void)[] = [];
  const call = (t: T, fn: (t: T) => void) => {
    try {
      fn(t);
    } catch (err) {
      onError(asError(err));
    }
  };
  loading.then(
    (t) => {
      if (closed) return t.close();
      ready = t;
      for (const fn of queue) call(t, fn);
      queue = [];
    },
    (err) => closed || onError(asError(err)),
  );
  return {
    run: (fn: (t: T) => void) => {
      if (closed) return;
      if (ready) call(ready, fn);
      else queue.push(fn);
    },
    close: () => {
      if (closed) return;
      closed = true;
      queue = [];
      ready?.close();
    },
  };
};

const webCodecsEncoder = (config: EncoderConfig, sink: EncoderSink): VoiceEncoder => {
  const e = new AudioEncoder({
    output: (chunk) => {
      const opus = new Uint8Array(chunk.byteLength);
      chunk.copyTo(opus);
      sink.output(opus);
    },
    error: sink.error,
  });
  e.configure(config);
  const live = () => e.state === "configured";
  return {
    encode: (samples, timestamp) => {
      if (!live()) return;
      const { numberOfChannels } = config;
      const data = new AudioData({
        format: "f32-planar",
        sampleRate: SAMPLE_RATE,
        numberOfFrames: samples.length / numberOfChannels,
        numberOfChannels,
        timestamp,
        data: samples,
      });
      e.encode(data);
      data.close();
    },
    flush: () => void (live() && e.flush().catch(() => {})), // closed meanwhile
    // close() throws on an encoder that already failed and closed itself.
    close: () => e.state !== "closed" && e.close(),
  };
};

const webCodecsDecoder = (sink: DecoderSink): VoiceDecoder => {
  const d = new AudioDecoder({
    output: (data) => {
      const planes = Array.from({ length: data.numberOfChannels }, (_, ch) => {
        const plane = new Float32Array(data.numberOfFrames);
        data.copyTo(plane, { planeIndex: ch, format: "f32-planar" });
        return plane;
      });
      const at = data.timestamp;
      data.close();
      sink.output(planes, at);
    },
    error: sink.error,
  });
  d.configure(DECODER_CONFIG);
  return {
    decode: (opus, timestamp) => d.decode(new EncodedAudioChunk({ type: "key", timestamp, data: opus })),
    close: () => d.state !== "closed" && d.close(),
  };
};

// libopus is synchronous: each frame gives its packet at once.
const wasmEncoder = async (config: EncoderConfig, sink: EncoderSink): Promise<VoiceEncoder> => {
  const { createEncoder } = await import("libopus-wasm");
  const e = await createEncoder(libopusOptions(config));
  return {
    encode: (samples) => sink.output(e.encodeFloat(interleave(samples, config.numberOfChannels))),
    flush: () => {},
    close: () => e.free(),
  };
};

const wasmDecoder = async (sink: DecoderSink): Promise<VoiceDecoder> => {
  const { createDecoder } = await import("libopus-wasm");
  const d = await createDecoder({ channels: 2, sampleRate: SAMPLE_RATE });
  return {
    decode: (opus, timestamp) => sink.output(deinterleave(d.decodeFloat(opus) as Float32Array<ArrayBuffer>, 2), timestamp),
    close: () => d.free(),
  };
};

// Chosen once per page.
let chosen: Promise<CodecKind> | null = null;
const codec = () =>
  (chosen ??= chooseCodec().then((kind) => {
    console.info("voice codec:", kind);
    return kind;
  }));

export const voiceEncoder = (config: EncoderConfig, sink: EncoderSink): VoiceEncoder => {
  const r = whenReady(
    codec().then((kind) => (kind === "webcodecs" ? webCodecsEncoder(config, sink) : wasmEncoder(config, sink))),
    sink.error,
  );
  return { encode: (samples, timestamp) => r.run((e) => e.encode(samples, timestamp)), flush: () => r.run((e) => e.flush()), close: r.close };
};

export const voiceDecoder = (sink: DecoderSink): VoiceDecoder => {
  const r = whenReady(
    codec().then((kind) => (kind === "webcodecs" ? webCodecsDecoder(sink) : wasmDecoder(sink))),
    sink.error,
  );
  return { decode: (opus, timestamp) => r.run((d) => d.decode(opus, timestamp)), close: r.close };
};
