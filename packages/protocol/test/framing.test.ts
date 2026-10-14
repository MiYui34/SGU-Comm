import { describe, expect, it } from "vitest";
import { decodeFrames, encodeFrame, MAX_PAYLOAD } from "../src/framing";

const bytes = (...xs: number[]) => new Uint8Array(xs);

describe("encodeFrame", () => {
  it("writes a 6-byte big-endian header before the payload", () => {
    expect(encodeFrame(9, bytes(0xaa, 0xbb))).toEqual(
      bytes(0x00, 0x09, 0x00, 0x00, 0x00, 0x02, 0xaa, 0xbb),
    );
  });

  it("encodes an empty payload", () => {
    expect(encodeFrame(3, bytes())).toEqual(bytes(0, 3, 0, 0, 0, 0));
  });
});

describe("decodeFrames", () => {
  it("decodes one complete frame", () => {
    const { frames, rest } = decodeFrames(encodeFrame(7, bytes(1, 2, 3)));
    expect(frames).toEqual([{ type: 7, payload: bytes(1, 2, 3) }]);
    expect(rest).toEqual(bytes());
  });

  it("decodes several frames from one buffer", () => {
    const buf = new Uint8Array([
      ...encodeFrame(1, bytes(9)),
      ...encodeFrame(2, bytes()),
    ]);
    expect(decodeFrames(buf).frames).toEqual([
      { type: 1, payload: bytes(9) },
      { type: 2, payload: bytes() },
    ]);
  });

  it("keeps an incomplete header as rest", () => {
    const { frames, rest } = decodeFrames(bytes(0, 1, 0));
    expect(frames).toEqual([]);
    expect(rest).toEqual(bytes(0, 1, 0));
  });

  it("keeps an incomplete payload as rest", () => {
    const full = encodeFrame(5, bytes(1, 2, 3, 4));
    const { frames, rest } = decodeFrames(full.subarray(0, 8));
    expect(frames).toEqual([]);
    expect(rest).toEqual(full.subarray(0, 8));
  });

  it("returns complete frames and keeps the trailing partial one", () => {
    const buf = new Uint8Array([...encodeFrame(1, bytes(7)), 0, 2]);
    const { frames, rest } = decodeFrames(buf);
    expect(frames).toEqual([{ type: 1, payload: bytes(7) }]);
    expect(rest).toEqual(bytes(0, 2));
  });

  it("rejects a frame that is larger than the limit", () => {
    const header = new Uint8Array(6);
    new DataView(header.buffer).setUint32(2, MAX_PAYLOAD + 1);
    expect(() => decodeFrames(header)).toThrow(/too large/);
  });

  it("returns payloads that do not share memory with the input", () => {
    const buf = encodeFrame(1, bytes(5));
    const { frames } = decodeFrames(buf);
    buf[6] = 0;
    expect(frames[0]?.payload).toEqual(bytes(5));
  });
});
