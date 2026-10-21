import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { closeVoiceOutput, voiceOutput } from "../src/lib/voice/output";

// Android: the voice mix plays through an <audio> element (see output.ts).
class FakeAudio {
  static all: FakeAudio[] = [];
  paused = true;
  srcObject: unknown = "stream";
  onplaying: (() => void) | null = null;
  onpause: (() => void) | null = null;
  constructor() {
    FakeAudio.all.push(this);
  }
  play = vi.fn(async () => {
    this.paused = false;
    this.onplaying?.();
  });
  pause = vi.fn(() => {
    this.paused = true;
    this.onpause?.();
  });
}
const node = () => ({ connect: vi.fn(), disconnect: vi.fn() });
const fakeContext = () => {
  const input = node();
  return { input, ctx: { destination: {}, createGain: () => input, createMediaStreamDestination: () => ({ stream: {} }) } as unknown as AudioContext };
};
const listeners = new Set<unknown>();

beforeEach(() => {
  FakeAudio.all = [];
  listeners.clear();
  vi.stubGlobal("Audio", FakeAudio);
  vi.stubGlobal("MediaMetadata", class {});
  vi.stubGlobal("navigator", { userAgent: "Android", mediaSession: { setActionHandler() {} } });
  vi.stubGlobal("document", { addEventListener: (_: string, l: unknown) => listeners.add(l), removeEventListener: (_: string, l: unknown) => listeners.delete(l) });
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("closeVoiceOutput", () => {
  it("stops the old context's element and its retry on touch, and forgets the output", () => {
    const { ctx, input } = fakeContext();
    const out = voiceOutput(ctx);
    out.setVoice({ metadata: { title: "x" }, actions: {} } as never, () => {});
    const element = FakeAudio.all[0]!;
    expect(element.play).toHaveBeenCalled();
    closeVoiceOutput(ctx);
    expect(element.paused).toBe(true);
    expect(element.srcObject).toBeNull();
    expect(listeners.size).toBe(0);
    expect(input.disconnect).toHaveBeenCalled();
    expect(voiceOutput(ctx)).not.toBe(out);
  });

  it("drops a pending release: it must not end the media session the new output took over", () => {
    vi.useFakeTimers();
    const { ctx } = fakeContext();
    const out = voiceOutput(ctx);
    const media = navigator.mediaSession as unknown as { playbackState?: string };
    out.setVoice({ metadata: { title: "x" }, actions: {} } as never, () => {});
    out.release();
    closeVoiceOutput(ctx);
    media.playbackState = "playing"; // the new output's session
    vi.advanceTimersByTime(5000);
    expect(media.playbackState).toBe("playing");
  });

  it("ignores calls after it closed (the old client's teardown runs later)", () => {
    vi.useFakeTimers();
    const { ctx } = fakeContext();
    const out = voiceOutput(ctx);
    const element = FakeAudio.all[0]!;
    closeVoiceOutput(ctx);
    const media = navigator.mediaSession as unknown as { playbackState?: string };
    media.playbackState = "playing"; // the new output's session
    out.release();
    out.setVoice({ metadata: { title: "x" }, actions: {} } as never, () => {});
    vi.advanceTimersByTime(5000);
    expect(media.playbackState).toBe("playing");
    expect(element.play).not.toHaveBeenCalled();
    expect(element.onpause).toBeNull();
  });

  it("does nothing for a context without an output", () => {
    expect(() => closeVoiceOutput(fakeContext().ctx)).not.toThrow();
  });
});
