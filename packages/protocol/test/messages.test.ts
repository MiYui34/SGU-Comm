import { isFieldSet } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import { decodeFrames, encodeFrame } from "../src/framing";
import { UserStateSchema } from "../src/gen/Mumble_pb";
import { decodeMessage, encodeMessage, encodeVoicePacket } from "../src/messages";

const roundtrip = (bytes: Uint8Array) => {
  const { frames } = decodeFrames(bytes);
  expect(frames).toHaveLength(1);
  return decodeMessage(frames[0]!);
};

describe("messages", () => {
  it("uses the Mumble type IDs", () => {
    const ids = [
      encodeMessage("Version", {}),
      encodeMessage("Authenticate", {}),
      encodeMessage("Ping", {}),
      encodeMessage("UserState", {}),
      encodeMessage("TextMessage", { message: "hi" }),
      encodeMessage("PluginDataTransmission", {}),
    ].map((bytes) => decodeFrames(bytes).frames[0]?.type);
    expect(ids).toEqual([0, 2, 3, 9, 11, 26]);
  });

  it("round-trips a protobuf message", () => {
    const msg = roundtrip(
      encodeMessage("Authenticate", { username: "alice", opus: true }),
    );
    expect(msg.type).toBe("Authenticate");
    if (msg.type !== "Authenticate") return;
    expect(msg.message.username).toBe("alice");
    expect(msg.message.opus).toBe(true);
  });

  it("keeps field presence for partial updates", () => {
    const msg = roundtrip(encodeMessage("UserState", { session: 4, selfMute: false }));
    if (msg.type !== "UserState") throw new Error("wrong type");
    expect(isFieldSet(msg.message, UserStateSchema.field.selfMute)).toBe(true);
    expect(isFieldSet(msg.message, UserStateSchema.field.name)).toBe(false);
  });

  it("treats UDPTunnel as raw voice bytes", () => {
    const packet = new Uint8Array([0x80, 1, 2, 3]);
    expect(roundtrip(encodeVoicePacket(packet))).toEqual({ type: "UDPTunnel", packet });
  });

  it("does not throw on unknown message types", () => {
    const payload = new Uint8Array([1, 2]);
    expect(roundtrip(encodeFrame(200, payload))).toEqual({
      type: "Unknown",
      id: 200,
      payload,
    });
  });
});
