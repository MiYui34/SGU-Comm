import { describe, expect, it } from "vitest";
import { decodeFrame, encodeFrame, fitSize, pickCodec } from "../src/lib/stream/protocol";

describe("stream frames", () => {
  it("go up with flags and a timestamp, and come down with the streamer's session", () => {
    const up = encodeFrame(true, 1_500_000, new Uint8Array([7, 8]));
    expect([...up]).toEqual([1, 0, 0, 0, 0, 0, 0x16, 0xe3, 0x60, 7, 8]);
    const down = new Uint8Array([0, 0, 0, 5, ...up]);
    const f = decodeFrame(down.buffer);
    expect(f && { ...f, data: [...f.data] }).toEqual({ session: 5, key: true, timestamp: 1_500_000, data: [7, 8] });
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

describe("pickCodec", () => {
  it("takes the first codec the browser can encode", async () => {
    expect(await pickCodec(async () => true)).toBe("vp8");
    expect(await pickCodec(async (c) => c.startsWith("avc1"))).toMatch(/^avc1/);
    expect(await pickCodec(async () => false)).toBeNull();
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
