import { describe, expect, it } from "vitest";
import { decodeFrames } from "../src/framing";
import { decodeMessage, encodeMessage } from "../src/messages";
import { createMessageStream, handshake, ping, versionNumbers } from "../src/session";

const decodeAll = (bytes: Uint8Array) => decodeFrames(bytes).frames.map(decodeMessage);

describe("versionNumbers", () => {
  it("encodes the legacy and the new version format", () => {
    expect(versionNumbers(1, 5, 7)).toEqual({
      versionV1: 0x010507,
      versionV2: (1n << 48n) | (5n << 32n) | (7n << 16n),
    });
  });
});

describe("handshake", () => {
  it("sends Version, then Authenticate with Opus enabled", () => {
    const msgs = decodeAll(handshake({ username: "alice", password: "pw" }));
    expect(msgs.map((m) => m.type)).toEqual(["Version", "Authenticate"]);
    const [version, auth] = msgs;
    if (version?.type !== "Version" || auth?.type !== "Authenticate") throw new Error();
    expect(version.message.versionV1).toBe(0x010500);
    expect(version.message.release).toBe("Kallisti");
    expect(auth.message).toMatchObject({ username: "alice", password: "pw", opus: true });
  });
});

describe("ping", () => {
  it("sends the timestamp", () => {
    const [msg] = decodeAll(ping(1234));
    if (msg?.type !== "Ping") throw new Error();
    expect(msg.message.timestamp).toBe(1234n);
  });

  it("sends the connection stats", () => {
    const [msg] = decodeAll(ping(1234, { good: 90, late: 8, lost: 2, tcpPackets: 6, tcpPingAvg: 42.5, tcpPingVar: 16 }));
    if (msg?.type !== "Ping") throw new Error();
    expect(msg.message).toMatchObject({ good: 90, late: 8, lost: 2, tcpPackets: 6, tcpPingAvg: 42.5, tcpPingVar: 16 });
  });
});

describe("createMessageStream", () => {
  it("decodes messages split across chunks", () => {
    const push = createMessageStream();
    const bytes = new Uint8Array([
      ...encodeMessage("Ping", { timestamp: 1n }),
      ...encodeMessage("Ping", { timestamp: 2n }),
    ]);
    const a = push(bytes.subarray(0, 5));
    const b = push(bytes.subarray(5, 12));
    const c = push(bytes.subarray(12));
    expect(a).toEqual([]);
    expect([...b, ...c].map((m) => m.type)).toEqual(["Ping", "Ping"]);
  });
});
