import { describe, expect, it, vi } from "vitest";
import { createVoiceAudio } from "../src/lib/voice/voiceAudio.svelte";

const ctxA = { name: "a" } as unknown as AudioContext;
const ctxB = { name: "b" } as unknown as AudioContext;
const parts = (ctx: AudioContext) => ({ playback: { ctx, close: vi.fn() }, capture: { ctx, close: vi.fn() } });

describe("createVoiceAudio", () => {
  it("has no playback or capture without a context (no voice support)", () => {
    const make = vi.fn(parts);
    const voice = createVoiceAudio(null, make);
    voice.move(ctxB);
    expect(voice.playback).toBeNull();
    expect(voice.capture).toBeNull();
    expect(make).not.toHaveBeenCalled();
  });

  it("moves playback and capture to a new context and closes the old ones", () => {
    const voice = createVoiceAudio(ctxA, parts);
    const [oldPlayback, oldCapture] = [voice.playback!, voice.capture!];
    voice.move(ctxB);
    expect(oldCapture.close).toHaveBeenCalled();
    expect(oldPlayback.close).toHaveBeenCalled();
    expect(voice.playback!.ctx).toBe(ctxB);
    expect(voice.capture!.ctx).toBe(ctxB);
  });

  it("does not move after it closed (a restart after the connection went)", () => {
    const make = vi.fn(parts);
    const voice = createVoiceAudio(ctxA, make);
    voice.close();
    expect(voice.capture!.close).toHaveBeenCalled();
    expect(voice.playback!.close).toHaveBeenCalled();
    voice.move(ctxB);
    expect(make).toHaveBeenCalledTimes(1);
  });
});
