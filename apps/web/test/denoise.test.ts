import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { describe, expect, it } from "vitest";
// The worklet's helpers (the file registers its processor only in a worklet).
import { createDenoiser, Fifo } from "../src/lib/voice/capture.worklet.js";

const wasm = new WebAssembly.Module(readFileSync(createRequire(import.meta.url).resolve("@jitsi/rnnoise-wasm/dist/rnnoise.wasm")));

// Deterministic white noise.
const noise = (n: number, amp: number, seed = 1) => {
  const out = new Float32Array(n);
  for (let i = 0; i < n; i++) {
    seed = (seed * 1103515245 + 12345) & 0x7fffffff;
    out[i] = ((seed / 0x7fffffff) * 2 - 1) * amp;
  }
  return out;
};
const rmsDb = (b: Float32Array) => 10 * Math.log10(b.reduce((s, x) => s + x * x, 0) / b.length);

describe("RNNoise in the capture worklet", () => {
  // RNNoise adapts within a second; synthetic white noise gets about 8 dB
  // quieter (real keyboard and fan noise differ). A broken wrapper gives
  // 0 dB or garbage.
  it("makes steady noise quieter and sees no voice in it", () => {
    const d = createDenoiser(wasm);
    const input = noise(96_000, 0.05);
    const out = input.slice();
    const vads: number[] = [];
    for (let i = 0; i < out.length; i += 480) vads.push(d.process(out.subarray(i, i + 480)));
    d.destroy();
    const half = 48_000;
    expect(rmsDb(out.subarray(half))).toBeLessThan(rmsDb(input.subarray(half)) - 6);
    expect(Math.max(...vads.slice(vads.length / 2))).toBeLessThan(0.5);
  });

  it("refuses frames of another size", () => {
    const d = createDenoiser(wasm);
    expect(() => d.process(new Float32Array(100))).toThrow();
    d.destroy();
  });
});

describe("Fifo (the echo test's output)", () => {
  it("gives back what went in, then silence", () => {
    const f = new Fifo(8);
    f.push(Float32Array.of(1, 2, 3));
    const out = new Float32Array(4);
    f.pull(out);
    expect([...out]).toEqual([1, 2, 3, 0]);
    // Too much: the oldest go (a late reader hears the newest).
    f.push(new Float32Array(10).fill(5));
    f.pull(out);
    expect([...out]).toEqual([5, 5, 5, 5]);
  });
});
