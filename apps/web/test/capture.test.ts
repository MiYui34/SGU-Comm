import { afterEach, describe, expect, it, vi } from "vitest";
import { createCapture } from "../src/lib/voice/capture.svelte";

// Just enough of the browser's media APIs for start().
const fakeNode = () => ({ connect: vi.fn((n: unknown) => n), disconnect: vi.fn(), port: { postMessage: vi.fn() } });

const setup = () => {
  const tracks: { stop: ReturnType<typeof vi.fn> }[] = [];
  const getUserMedia = vi.fn(async () => {
    const track = { stop: vi.fn(), label: "mic", onended: null, getSettings: () => ({ channelCount: 2 }) };
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
  return { tracks, getUserMedia, addModule, capture: createCapture(ctx, () => {}) };
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

  it("opens the microphone again without filters for music mode", async () => {
    const { getUserMedia, capture } = setup();
    await capture.start("mic-2");
    capture.setMusicMode(true);
    await vi.waitFor(() => expect(getUserMedia).toHaveBeenCalledTimes(2));
    expect(getUserMedia.mock.lastCall).toEqual([
      { audio: { deviceId: { exact: "mic-2" }, echoCancellation: false, noiseSuppression: false, autoGainControl: false, channelCount: { ideal: 2 } } },
    ]);
    // The microphone has two channels: the worklet collects both.
    const node = vi.mocked(AudioWorkletNode).mock.results[0]!.value as ReturnType<typeof fakeNode>;
    await vi.waitFor(() => expect(node.port.postMessage).toHaveBeenCalledWith({ channels: 2 }));
  });

  it("opens the microphone again when music mode changes while it opens", async () => {
    const { getUserMedia, tracks, capture } = setup();
    const opening = capture.start();
    capture.setMusicMode(true); // during the permission prompt
    await opening;
    await vi.waitFor(() => expect(getUserMedia).toHaveBeenCalledTimes(2));
    expect(getUserMedia.mock.lastCall).toMatchObject([{ audio: { autoGainControl: false } }]);
    await vi.waitFor(() => expect(tracks.map((t) => t.stop.mock.calls.length)).toEqual([1, 0]));
  });
});
