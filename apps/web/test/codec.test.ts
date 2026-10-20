import { describe, expect, it, vi } from "vitest";
import {
  chooseCodec,
  decoderKind,
  deinterleave,
  interleave,
  libopusOptions,
  voiceDecoder,
  voiceEncoder,
  whenReady,
} from "../src/lib/voice/codec";
import { encoderConfig } from "../src/lib/voice/opus";

// WebCodecs stand-ins: isConfigSupported answers per codec config.
const webCodecs = (encoder: boolean | Error, decoder = true) => ({
  AudioEncoder: {
    isConfigSupported: async () => {
      if (encoder instanceof Error) throw encoder;
      return { supported: encoder };
    },
  },
  AudioDecoder: { isConfigSupported: async () => ({ supported: decoder }) },
});

describe("chooseCodec", () => {
  it("takes WebCodecs when it has Opus", async () => {
    expect(await chooseCodec(webCodecs(true))).toBe("webcodecs");
  });
  it("takes WebAssembly without WebCodecs audio", async () => {
    expect(await chooseCodec({})).toBe("wasm");
    expect(await chooseCodec({ AudioDecoder: webCodecs(true).AudioDecoder })).toBe("wasm");
  });
  it("takes WebAssembly when WebCodecs has no Opus (WebKitGTK without its GStreamer plugins)", async () => {
    expect(await chooseCodec(webCodecs(false))).toBe("wasm");
    expect(await chooseCodec(webCodecs(true, false))).toBe("wasm");
    expect(await chooseCodec(webCodecs(new Error("no")))).toBe("wasm");
  });
});

describe("decoderKind", () => {
  it("decodes with WebAssembly in the Linux app: WebKitGTK's AudioData.copyTo leaks", () => {
    expect(decoderKind("webcodecs", true)).toBe("wasm");
    expect(decoderKind("wasm", true)).toBe("wasm");
  });
  it("keeps the codec elsewhere", () => {
    expect(decoderKind("webcodecs", false)).toBe("webcodecs");
    expect(decoderKind("wasm", false)).toBe("wasm");
  });
});

describe("interleave", () => {
  it("leaves mono as it is", () => {
    const mono = new Float32Array([1, 2, 3]);
    expect(interleave(mono, 1)).toBe(mono);
    expect(deinterleave(mono, 1)).toEqual([mono]);
  });
  it("turns planar stereo into interleaved and back", () => {
    const planar = new Float32Array([1, 2, 3, 10, 20, 30]);
    const mixed = interleave(planar, 2);
    expect([...mixed]).toEqual([1, 10, 2, 20, 3, 30]);
    expect(deinterleave(mixed, 2).map((p) => [...p])).toEqual([
      [1, 2, 3],
      [10, 20, 30],
    ]);
  });
});

describe("libopusOptions", () => {
  it("encodes speech like WebCodecs does", () => {
    expect(libopusOptions(encoderConfig(false, 1))).toEqual({
      channels: 1,
      sampleRate: 48_000,
      frameSize: 960,
      bitrate: 40_000,
      application: 2049, // audio: WebCodecs' default
      signal: -1000, // auto
    });
  });
  it("tunes music", () => {
    expect(libopusOptions(encoderConfig(true, 2))).toMatchObject({ channels: 2, bitrate: 128_000, application: 2049, signal: 3002, complexity: 10 });
  });
});

describe("whenReady", () => {
  const fake = () => ({ calls: [] as number[], closed: false, close() { this.closed = true; } });
  const later = <T,>() => {
    let resolve!: (t: T) => void, reject!: (e: unknown) => void;
    const p = new Promise<T>((res, rej) => ((resolve = res), (reject = rej)));
    return { p, resolve, reject };
  };

  it("runs calls made while loading once it is there, in order", async () => {
    const { p, resolve } = later<ReturnType<typeof fake>>();
    const r = whenReady(p, () => {});
    r.run((c) => c.calls.push(1));
    r.run((c) => c.calls.push(2));
    const c = fake();
    resolve(c);
    await p;
    r.run((c) => c.calls.push(3));
    expect(c.calls).toEqual([1, 2, 3]);
  });
  it("closes one that arrives after close", async () => {
    const { p, resolve } = later<ReturnType<typeof fake>>();
    const r = whenReady(p, () => {});
    r.run((c) => c.calls.push(1));
    r.close();
    const c = fake();
    resolve(c);
    await p;
    expect(c).toMatchObject({ calls: [], closed: true });
  });
  it("puts calls made while the queue runs behind it", async () => {
    const { p, resolve } = later<ReturnType<typeof fake>>();
    const r = whenReady(p, () => {});
    r.run((c) => {
      c.calls.push(1);
      r.run((c) => c.calls.push(3));
    });
    r.run((c) => c.calls.push(2));
    const c = fake();
    resolve(c);
    await p;
    expect(c.calls).toEqual([1, 2, 3]);
  });
  it("stops running queued calls when one closes it", async () => {
    const { p, resolve } = later<ReturnType<typeof fake>>();
    const r = whenReady(p, () => {});
    r.run((c) => c.calls.push(1));
    r.run(() => r.close());
    r.run((c) => c.calls.push(2));
    const c = fake();
    resolve(c);
    await p;
    expect(c).toMatchObject({ calls: [1], closed: true });
  });
  it("closes once", async () => {
    const c = fake();
    const close = vi.spyOn(c, "close");
    const r = whenReady(Promise.resolve(c), () => {});
    await Promise.resolve();
    r.close();
    r.close();
    expect(close).toHaveBeenCalledTimes(1);
  });
  it("reports a failed load and failed calls", async () => {
    const errors: string[] = [];
    const { p, reject } = later<ReturnType<typeof fake>>();
    whenReady(p, (e) => errors.push(e.message));
    reject(new Error("load"));
    await p.catch(() => {});
    const r = whenReady(Promise.resolve(fake()), (e) => errors.push(e.message));
    await Promise.resolve();
    r.run(() => {
      throw new Error("call");
    });
    expect(errors).toEqual(["load", "call"]);
  });
});

describe("a failed codec", () => {
  const later = <T,>() => {
    let resolve!: (t: T) => void, reject!: (e: unknown) => void;
    const p = new Promise<T>((res, rej) => ((resolve = res), (reject = rej)));
    return { p, resolve, reject };
  };
  const fake = () => ({ calls: [] as number[], closed: false, close() { this.closed = true; } });

  it("takes no more calls after a failed load, drops the queued ones and reports once", async () => {
    const errors: string[] = [];
    const { p, reject } = later<ReturnType<typeof fake>>();
    const r = whenReady(p, (e) => errors.push(e.message));
    const queued = vi.fn();
    r.run(queued);
    reject(new Error("load"));
    await p.catch(() => {});
    expect(r.failed()).toBe(true);
    r.run(queued);
    expect(queued).not.toHaveBeenCalled();
    expect(errors).toEqual(["load"]);
  });
  it("stops at a failed call: closes the codec, runs nothing after it, reports once", async () => {
    const errors: string[] = [];
    const c = fake();
    const r = whenReady(Promise.resolve(c), (e) => errors.push(e.message));
    await Promise.resolve();
    r.run(() => {
      throw new Error("call");
    });
    r.run((c) => c.calls.push(1));
    expect(c).toMatchObject({ calls: [], closed: true });
    expect(errors).toEqual(["call"]);
  });
  it("fails on the codec's own error, also while calls wait", async () => {
    const errors: string[] = [];
    const { p, resolve } = later<ReturnType<typeof fake>>();
    const r = whenReady(p, (e) => errors.push(e.message));
    r.run((c) => c.calls.push(1));
    r.fail(new Error("encoder"));
    r.fail(new Error("again"));
    const c = fake();
    resolve(c);
    await p;
    expect(c).toMatchObject({ calls: [], closed: true });
    expect(errors).toEqual(["encoder"]);
  });
});

// Node has no WebCodecs: the real WebAssembly path, end to end.
describe("the WebAssembly codec", () => {
  it("encodes a frame and decodes it to two channels of 20 ms", async () => {
    const tone = new Float32Array(960).map((_, i) => Math.sin(i / 8) * 0.5);
    const packet = await new Promise<Uint8Array>((resolve, reject) => {
      const e = voiceEncoder(encoderConfig(false, 1), { output: resolve, error: reject });
      e.encode(tone, 0);
      e.flush();
    });
    expect(packet.length).toBeGreaterThan(0);
    const [planes, at] = await new Promise<[Float32Array[], number]>((resolve, reject) => {
      const d = voiceDecoder({ output: (p, t) => resolve([p, t]), error: reject });
      d.decode(packet, 20_000);
    });
    expect(planes.map((p) => p.length)).toEqual([960, 960]);
    expect(at).toBe(20_000);
  });
});
