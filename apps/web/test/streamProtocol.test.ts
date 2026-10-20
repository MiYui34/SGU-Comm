import { describe, expect, it } from "vitest";
import { cannotPlay, decodeFrame, decodeWatch, encodeFrame, fitSize, pickCodec, previewSize } from "../src/lib/stream/protocol";

describe("stream frames", () => {
  it("go up with flags and a timestamp, and come down with the streamer's session", () => {
    const up = encodeFrame(true, 1_500_000, new Uint8Array([7, 8]));
    expect([...up]).toEqual([1, 0, 0, 0, 0, 0, 0x16, 0xe3, 0x60, 7, 8]);
    const down = new Uint8Array([0, 0, 0, 5, ...up]);
    const f = decodeFrame(down.buffer);
    expect(f && { ...f, data: [...f.data] }).toEqual({ session: 5, key: true, sound: false, timestamp: 1_500_000, data: [7, 8] });
    expect(decodeFrame(new Uint8Array(5).buffer)).toBeNull();
  });
});

describe("fitSize", () => {
  it("keeps the aspect, fits 1920x1080 and makes even sizes", () => {
    expect(fitSize(3840, 2160)).toEqual({ width: 1920, height: 1080 });
    expect(fitSize(1001, 601)).toEqual({ width: 1000, height: 600 });
    expect(fitSize(1080, 2400)).toEqual({ width: 486, height: 1080 });
  });
});

describe("previewSize", () => {
  it("is 320 wide, at most 1024 high (the server's limit)", () => {
    expect(previewSize(1920, 1080)).toEqual({ width: 320, height: 180 });
    expect(previewSize(1080, 1920)).toEqual({ width: 320, height: 569 });
    expect(previewSize(300, 1080)).toEqual({ width: 284, height: 1024 });
    expect(previewSize(0, 0)).toEqual({ width: 320, height: 1 });
  });
});

describe("pickCodec", () => {
  it("takes the first codec the browser can encode", async () => {
    expect(await pickCodec(async () => true)).toBe("vp8");
    expect(await pickCodec(async (c) => c.startsWith("avc1"))).toMatch(/^avc1/);
    expect(await pickCodec(async () => false)).toBeNull();
  });
});

describe("decoder failures", () => {
  it("give up after errors in a row without a picture", () => {
    const watch = decodeWatch(3);
    expect(watch.failed()).toBe(false);
    expect(watch.failed()).toBe(false);
    watch.decoded(); // a picture: the count starts over
    expect(watch.failed()).toBe(false);
    expect(watch.failed()).toBe(false);
    expect(watch.failed()).toBe(true);
  });

  it("say why for H.264 (streams from OBS)", () => {
    expect(cannotPlay("avc1.640028")).toMatch(/H\.264/);
    expect(cannotPlay("vp8")).toBe("This browser cannot play this stream.");
  });
});

import { initialTextState, parseServerEvent, reduceText } from "../src/lib/textChannels";

describe("live streams in the text state", () => {
  it("come with ready and with streams events", () => {
    const ev = (v: object) => parseServerEvent(JSON.stringify(v))!;
    let s = reduceText(initialTextState, ev({ type: "ready", channels: [], streams: [{ session: 4, channel: 2 }] }));
    expect(s.streams).toEqual([{ session: 4, channel: 2 }]);
    s = reduceText(s, ev({ type: "streams", streams: [] }));
    expect(s.streams).toEqual([]);
    expect(ev({ type: "stream-token", token: "abc" })).toEqual({ type: "stream-token", token: "abc" });
  });
});

describe("stream previews in the text state", () => {
  it("come with ready and with stream-preview events, and survive list changes", () => {
    const ev = (v: object) => parseServerEvent(JSON.stringify(v))!;
    let s = reduceText(initialTextState, ev({ type: "ready", channels: [], streams: [{ session: 4, channel: 2, preview: "data:image/jpeg;base64,AA" }] }));
    expect(s.streamPreviews[4]).toBe("data:image/jpeg;base64,AA");
    s = reduceText(s, ev({ type: "stream-preview", session: 4, image: "data:image/jpeg;base64,BB" }));
    expect(s.streamPreviews[4]).toBe("data:image/jpeg;base64,BB");
    s = reduceText(s, ev({ type: "streams", streams: [{ session: 4, channel: 2 }] }));
    expect(s.streamPreviews[4]).toBe("data:image/jpeg;base64,BB");
    s = reduceText(s, ev({ type: "streams", streams: [] }));
    expect(s.streamPreviews[4]).toBeUndefined();
    expect(ev({ type: "stream-preview", session: 4, image: "javascript:alert(1)" })).toBeNull();
  });
});

describe("sound frames", () => {
  it("carry flag bit 1", () => {
    const up = encodeFrame(false, 20_000, new Uint8Array([9]), true);
    expect(up[0]).toBe(2);
    expect(decodeFrame(new Uint8Array([0, 0, 0, 1, ...up]).buffer)?.sound).toBe(true);
  });
});

describe("cleared stream previews", () => {
  it("go away", () => {
    const ev = (v: object) => parseServerEvent(JSON.stringify(v))!;
    let s = reduceText(initialTextState, ev({ type: "ready", channels: [], streams: [{ session: 4, channel: 2, preview: "data:image/jpeg;base64,AA" }] }));
    s = reduceText(s, ev({ type: "stream-preview", session: 4, image: "" }));
    expect(s.streamPreviews[4]).toBeUndefined();
  });
});

import { resample } from "../src/lib/stream/client.svelte";

describe("resample", () => {
  it("turns 44.1 kHz into 48 kHz, keeping the shape", () => {
    const input = new Float32Array(441).map((_, i) => i / 441);
    const out = resample(input, 44_100, 48_000);
    expect(out.length).toBe(480);
    expect(out[0]).toBe(0);
    expect(out[240]).toBeCloseTo(0.5, 2);
  });
});
