import { describe, expect, it } from "vitest";
import { decodeFrame, encodeFrame, fitSize, pickCodec } from "../src/lib/stream/protocol";

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

import { createAvClock } from "../src/lib/stream/protocol";

describe("sound frames", () => {
  it("carry flag bit 1", () => {
    const up = encodeFrame(false, 20_000, new Uint8Array([9]), true);
    expect(up[0]).toBe(2);
    expect(decodeFrame(new Uint8Array([0, 0, 0, 1, ...up]).buffer)?.sound).toBe(true);
  });
});

describe("createAvClock", () => {
  it("plays sound a little ahead and places pictures on the same clock", () => {
    const clock = createAvClock(0.15);
    expect(clock.at(1_000_000, 10)).toBeCloseTo(10.15); // the first sound sets the clock
    expect(clock.at(1_020_000, 10.01)).toBeCloseTo(10.17);
    expect(clock.at(1_000_000, 10.2)).toBeNull(); // too late: dropped
  });
  it("starts over when it fell far behind", () => {
    const clock = createAvClock(0.15);
    clock.at(0, 0);
    expect(clock.at(1_000_000, 5)).toBeCloseTo(5.15);
  });
  it("knows nothing before the first sound", () => {
    expect(createAvClock(0.15).time(1_000_000)).toBeNull();
  });
});

describe("createAvClock recovery", () => {
  it("starts over after a few late sounds in a row (a delay that stays)", () => {
    const clock = createAvClock(0.15);
    clock.at(0, 0);
    let last: number | null = null;
    for (let i = 1; i <= 6; i++) last = clock.at(i * 20_000, 0.5 + i * 0.02); // each 0.3 s late
    expect(last).not.toBeNull();
  });
  it("starts over when sound would play far ahead", () => {
    const clock = createAvClock(0.15);
    clock.at(5_000_000, 10);
    expect(clock.at(5_020_000, 7)).toBeCloseTo(7.15); // the viewer's clock jumped back
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
