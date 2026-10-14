import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import type { Bytes } from "./framing";
import { AudioSchema } from "./gen/MumbleUDP_pb";
import { readVarint, writeVarint } from "./varint";

// Servers from 1.5 on use the protobuf format, older ones the legacy format.
export type AudioFormat = "protobuf" | "legacy";

export type AudioPacket = {
  // Client to server: the voice target (0 = normal talking).
  // Server to client: the context (0 = normal, 1 = shout, 2 = whisper, 3 = listener).
  target: number;
  session: number;
  frameNumber: number;
  opus: Uint8Array;
  terminator: boolean;
};

export type OutgoingAudio = Omit<AudioPacket, "session">;

const PROTOBUF_AUDIO = 0x00;
const LEGACY_OPUS = 4;
const LEGACY_TERMINATOR = 0x2000;
const LEGACY_LENGTH_MASK = 0x1fff;

const decodeProtobuf = (buf: Uint8Array): AudioPacket | null => {
  if (buf[0] !== PROTOBUF_AUDIO) return null;
  const msg = fromBinary(AudioSchema, buf.subarray(1));
  return {
    target: msg.Header.case === undefined ? 0 : msg.Header.value,
    session: msg.senderSession,
    frameNumber: Number(msg.frameNumber),
    opus: msg.opusData,
    terminator: msg.isTerminator,
  };
};

const decodeLegacy = (buf: Uint8Array): AudioPacket | null => {
  if (buf.length === 0 || buf[0]! >> 5 !== LEGACY_OPUS) return null;
  const session = readVarint(buf, 1);
  const frame = readVarint(buf, session.offset);
  const header = readVarint(buf, frame.offset);
  const length = header.value & LEGACY_LENGTH_MASK;
  const start = header.offset;
  if (start + length > buf.length) return null;
  return {
    target: buf[0]! & 0x1f,
    session: session.value,
    frameNumber: frame.value,
    opus: buf.slice(start, start + length),
    terminator: (header.value & LEGACY_TERMINATOR) !== 0,
  };
};

// Server to client. Returns null for anything that is not Opus audio,
// including broken packets: this is network input and must not throw.
export const decodeAudio = (format: AudioFormat, buf: Uint8Array): AudioPacket | null => {
  try {
    return format === "protobuf" ? decodeProtobuf(buf) : decodeLegacy(buf);
  } catch {
    return null;
  }
};

const concat = (parts: Uint8Array[]): Bytes => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  parts.reduce((offset, p) => (out.set(p, offset), offset + p.length), 0);
  return out;
};

// Client to server: the server fills in the session.
export const encodeAudio = (format: AudioFormat, p: OutgoingAudio): Bytes => {
  if (format === "protobuf") {
    const msg = create(AudioSchema, {
      Header: { case: "target", value: p.target },
      frameNumber: BigInt(p.frameNumber),
      opusData: p.opus,
      isTerminator: p.terminator,
    });
    return concat([Uint8Array.of(PROTOBUF_AUDIO), toBinary(AudioSchema, msg)]);
  }
  return concat([
    Uint8Array.of((LEGACY_OPUS << 5) | (p.target & 0x1f)),
    writeVarint(p.frameNumber),
    writeVarint(p.opus.length | (p.terminator ? LEGACY_TERMINATOR : 0)),
    p.opus,
  ]);
};
