import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import { decodeAudio, encodeAudio } from "../src/audio";
import { AudioSchema } from "../src/gen/MumbleUDP_pb";

const bytes = (...xs: number[]) => new Uint8Array(xs);
const opus = bytes(1, 2, 3);

describe("legacy format", () => {
  it("decodes an Opus packet from the server", () => {
    expect(decodeAudio("legacy", bytes(0x80, 0x05, 0x81, 0x2c, 0x03, 1, 2, 3))).toEqual({
      target: 0,
      session: 5,
      frameNumber: 300,
      opus,
      terminator: false,
    });
  });

  it("reads the terminator bit and the target bits", () => {
    expect(decodeAudio("legacy", bytes(0x82, 0x05, 0x01, 0xa0, 0x03, 1, 2, 3))).toMatchObject({
      target: 2,
      terminator: true,
      opus,
    });
  });

  it("ignores other codecs, pings and broken packets", () => {
    expect(decodeAudio("legacy", bytes(0x40, 0x05, 0x01, 0x00))).toBeNull(); // Speex
    expect(decodeAudio("legacy", bytes(0x20, 0x01))).toBeNull(); // ping
    expect(decodeAudio("legacy", bytes(0x80, 0x05, 0x01, 0x09, 1))).toBeNull(); // short data
    expect(decodeAudio("legacy", bytes(0x80, 0x81))).toBeNull(); // truncated varint
    expect(decodeAudio("legacy", bytes())).toBeNull();
  });

  it("encodes a client packet without a session", () => {
    const out = encodeAudio("legacy", { target: 0, frameNumber: 300, opus, terminator: false });
    expect([...out]).toEqual([0x80, 0x81, 0x2c, 0x03, 1, 2, 3]);
  });

  it("encodes the terminator bit", () => {
    const out = encodeAudio("legacy", { target: 0, frameNumber: 1, opus, terminator: true });
    expect([...out]).toEqual([0x80, 0x01, 0xa0, 0x03, 1, 2, 3]);
  });
});

describe("protobuf format", () => {
  it("decodes an audio packet from the server", () => {
    const body = toBinary(
      AudioSchema,
      create(AudioSchema, {
        Header: { case: "context", value: 1 },
        senderSession: 5,
        frameNumber: 300n,
        opusData: opus,
        isTerminator: true,
      }),
    );
    expect(decodeAudio("protobuf", new Uint8Array([0x00, ...body]))).toEqual({
      target: 1,
      session: 5,
      frameNumber: 300,
      opus,
      terminator: true,
    });
  });

  it("ignores pings and broken packets", () => {
    expect(decodeAudio("protobuf", bytes(0x01, 0x08, 0x01))).toBeNull();
    expect(decodeAudio("protobuf", bytes(0x00, 0xff, 0xff))).toBeNull();
    expect(decodeAudio("protobuf", bytes())).toBeNull();
  });

  it("encodes a client packet with the target in the header", () => {
    const out = encodeAudio("protobuf", { target: 0, frameNumber: 7, opus, terminator: false });
    expect(out[0]).toBe(0x00);
    const msg = fromBinary(AudioSchema, out.subarray(1));
    expect(msg.Header).toEqual({ case: "target", value: 0 });
    expect(msg.frameNumber).toBe(7n);
    expect(msg.opusData).toEqual(opus);
    expect(msg.isTerminator).toBe(false);
  });
});
