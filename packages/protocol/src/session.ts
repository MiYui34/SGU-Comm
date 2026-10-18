import { decodeFrames, type Bytes } from "./framing";
import type { MessageInitShape } from "@bufbuild/protobuf";
import type { PingSchema } from "./gen/Mumble_pb";
import { decodeMessage, encodeMessage, type MumbleMessage } from "./messages";

const concat = (...parts: Uint8Array[]): Bytes => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  parts.reduce((offset, p) => (out.set(p, offset), offset + p.length), 0);
  return out;
};

export const versionNumbers = (major: number, minor: number, patch: number) => ({
  versionV1: (major << 16) | (minor << 8) | patch,
  versionV2: (BigInt(major) << 48n) | (BigInt(minor) << 32n) | (BigInt(patch) << 16n),
});

export const handshake = ({
  username,
  password = "",
}: {
  username: string;
  password?: string;
}): Bytes =>
  concat(
    encodeMessage("Version", {
      ...versionNumbers(1, 5, 0),
      release: "Kallisti",
      os: "Web",
    }),
    encodeMessage("Authenticate", { username, password, opus: true }),
  );

// stats: what the client measured (good, late, lost, tcpPingAvg, ...), for
// the server to show.
export type PingStats = Omit<MessageInitShape<typeof PingSchema>, "timestamp" | "$typeName">;

export const ping = (now: number, stats: PingStats = {}): Bytes =>
  encodeMessage("Ping", { ...stats, timestamp: BigInt(now) });

// Turns a stream of byte chunks into messages. Keeps partial frames between calls.
export const createMessageStream = () => {
  let rest: Uint8Array = new Uint8Array();
  return (chunk: Uint8Array): MumbleMessage[] => {
    const decoded = decodeFrames(rest.length ? concat(rest, chunk) : chunk);
    rest = decoded.rest;
    return decoded.frames.map(decodeMessage);
  };
};
