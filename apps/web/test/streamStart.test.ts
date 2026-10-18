import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../src/lib/stream/ticker.worker?worker", () => ({ default: class {} }));
const { createStreamClient } = await import("../src/lib/stream/client.svelte");

// Starting a screen share while the connection goes away: the browser parts
// are fakes (a socket that is ready at once, a picker the test answers).
class FakeSocket {
  static OPEN = 1;
  static all: FakeSocket[] = [];
  readyState = 0;
  binaryType = "";
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  constructor() {
    FakeSocket.all.push(this);
    queueMicrotask(() => {
      this.readyState = 1;
      this.onopen?.();
      this.onmessage?.({ data: '{"type":"ready"}' });
    });
  }
  sent: string[] = [];
  send(data: string) {
    this.sent.push(data);
  }
  types() {
    return this.sent.map((d) => JSON.parse(d).type as string);
  }
  close() {
    if (this.readyState === 3) return;
    this.readyState = 3;
    queueMicrotask(() => this.onclose?.());
  }
}

class FakeEncoder {
  static made = 0;
  static isConfigSupported = async () => ({ supported: true });
  state = "configured";
  constructor() {
    FakeEncoder.made++;
  }
  configure() {}
  close() {
    this.state = "closed";
  }
}

const capture = () => {
  const track = { stop: vi.fn(), getSettings: () => ({}), onended: null as (() => void) | null };
  return { track, stream: { getVideoTracks: () => [track], getAudioTracks: () => [], getTracks: () => [track] } };
};

let pickers: ((s: unknown) => void)[] = [];
const flush = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  FakeSocket.all = [];
  FakeEncoder.made = 0;
  pickers = [];
  vi.stubGlobal("WebSocket", FakeSocket);
  vi.stubGlobal("VideoEncoder", FakeEncoder);
  // Frames of the shared screen: none come.
  const reader = { read: () => new Promise(() => {}), cancel: async () => {} };
  vi.stubGlobal("MediaStreamTrackProcessor", class { readable = { getReader: () => reader }; });
  vi.stubGlobal("navigator", { mediaDevices: { getDisplayMedia: () => new Promise((r) => pickers.push(r)) } });
});
afterEach(() => vi.unstubAllGlobals());

describe("starting a screen share", () => {
  it("stops the capture at once when the client closed while the picker was open", async () => {
    const client = createStreamClient("ws://x", async () => "t", null);
    const start = client.startSharing();
    await flush();
    expect(pickers).toHaveLength(1);
    client.close();
    await flush();
    const { track, stream } = capture();
    pickers[0]!(stream);
    await start;
    expect(track.stop).toHaveBeenCalled();
    expect(FakeEncoder.made).toBe(0);
    expect(client.sharing).toBe(false);
  });

  it("can start again after the connection dropped while the picker was open", async () => {
    const client = createStreamClient("ws://x", async () => "t", null);
    const start = client.startSharing();
    await flush();
    FakeSocket.all[0]!.close();
    await flush();
    const { track, stream } = capture();
    pickers[0]!(stream);
    await start;
    expect(track.stop).toHaveBeenCalled();
    void client.startSharing();
    await flush();
    expect(pickers).toHaveLength(2);
  });

  it("is not cancelled by a late reply to the share before", async () => {
    const client = createStreamClient("ws://x", async () => "t", null);
    const start = client.startSharing();
    await flush();
    FakeSocket.all[0]!.onmessage?.({ data: '{"type":"stopped"}' });
    pickers[0]!(capture().stream);
    await flush();
    expect(FakeSocket.all[0]!.types()).toContain("start");
    FakeSocket.all[0]!.onmessage?.({ data: '{"type":"started"}' });
    await start;
    expect(client.sharing).toBe(true);
  });

  it("tells the server when stopped after asking to start", async () => {
    const client = createStreamClient("ws://x", async () => "t", null);
    const start = client.startSharing();
    await flush();
    const { track, stream } = capture();
    pickers[0]!(stream);
    await flush();
    track.onended?.(); // the browser's "Stop sharing"
    await start;
    expect(FakeSocket.all[0]!.types()).toEqual(["auth", "start", "stop"]);
    expect(client.error).toBeNull();
  });
});
