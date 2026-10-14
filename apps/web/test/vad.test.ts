import { describe, expect, it } from "vitest";
import { levelDb, shouldTransmit, vadStep, type VadState } from "../src/lib/voice/vad";

describe("levelDb", () => {
  it("is 0 dBFS for a full-scale square wave", () => {
    expect(levelDb(new Float32Array([1, -1, 1, -1]))).toBeCloseTo(0);
  });

  it("is about -6 dBFS at half amplitude", () => {
    expect(levelDb(new Float32Array([0.5, -0.5]))).toBeCloseTo(-6.02, 1);
  });

  it("has a floor for silence", () => {
    expect(levelDb(new Float32Array(960))).toBe(-100);
  });
});

describe("vadStep", () => {
  const cfg = { thresholdDb: -45, holdMs: 300 };
  const quiet: VadState = { talking: false, silentMs: 0 };
  const run = (levels: number[]) =>
    levels.reduce<VadState>((s, db) => vadStep(s, db, 20, cfg), quiet);

  it("starts talking above the threshold", () => {
    expect(run([-30]).talking).toBe(true);
  });

  it("stays quiet below the threshold", () => {
    expect(run([-60, -50]).talking).toBe(false);
  });

  it("keeps talking during short pauses", () => {
    expect(run([-30, ...Array(14).fill(-60)]).talking).toBe(true); // 280 ms
  });

  it("stops after the hold time", () => {
    expect(run([-30, ...Array(15).fill(-60)]).talking).toBe(false); // 300 ms
  });
});

describe("shouldTransmit", () => {
  const base = { mode: "vad" as const, vadTalking: true, pttDown: false, muted: false };

  it("follows the VAD in voice activity mode", () => {
    expect(shouldTransmit(base)).toBe(true);
    expect(shouldTransmit({ ...base, vadTalking: false })).toBe(false);
  });

  it("follows the key in push-to-talk mode", () => {
    expect(shouldTransmit({ ...base, mode: "ptt", pttDown: false })).toBe(false);
    expect(shouldTransmit({ ...base, mode: "ptt", vadTalking: false, pttDown: true })).toBe(true);
  });

  it("never transmits when muted", () => {
    expect(shouldTransmit({ ...base, muted: true })).toBe(false);
    expect(shouldTransmit({ ...base, mode: "ptt", pttDown: true, muted: true })).toBe(false);
  });
});
