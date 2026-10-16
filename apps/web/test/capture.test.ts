import { afterEach, describe, expect, it, vi } from "vitest";
import { createCapture } from "../src/lib/voice/capture.svelte";

// Just enough of the browser's media APIs for start().
const fakeNode = () => ({ connect: vi.fn((n: unknown) => n), disconnect: vi.fn(), port: { postMessage: vi.fn() } });

const setup = () => {
  const tracks: { stop: ReturnType<typeof vi.fn> }[] = [];
  const getUserMedia = vi.fn(async () => {
    const track = { stop: vi.fn(), label: "mic", onended: null };
    tracks.push(track);
    return { getTracks: () => [track], getAudioTracks: () => [track] };
  });
  vi.stubGlobal("navigator", { mediaDevices: { getUserMedia } });
  vi.stubGlobal("GainNode", vi.fn(function () {
    return fakeNode();
  }));
  vi.stubGlobal("AudioWorkletNode", vi.fn(function () {
    return fakeNode();
  }));
  vi.stubGlobal("AudioEncoder", vi.fn(function () {
    return { configure: vi.fn(), close: vi.fn(), state: "configured" };
  }));
  const addModule = vi.fn();
  const ctx = {
    audioWorklet: { addModule },
    destination: {},
    createMediaStreamSource: vi.fn(() => fakeNode()),
  } as unknown as AudioContext;
  return { tracks, addModule, capture: createCapture(ctx, () => {}) };
};

describe("capture start", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("stops the microphone when the worklet fails to load", async () => {
    const { tracks, addModule, capture } = setup();
    addModule.mockRejectedValueOnce(new Error("no worklet"));
    await capture.start();
    expect(capture.micError).toBe("no worklet");
    expect(tracks.map((t) => t.stop.mock.calls.length)).toEqual([1]);
  });

  it("loads the worklet again after a failure", async () => {
    const { tracks, addModule, capture } = setup();
    addModule.mockRejectedValueOnce(new Error("no worklet"));
    await capture.start();
    await capture.start();
    expect(addModule).toHaveBeenCalledTimes(2);
    expect(capture.micError).toBeNull();
    expect(tracks.map((t) => t.stop.mock.calls.length)).toEqual([1, 0]); // the second one is in use
  });
});
