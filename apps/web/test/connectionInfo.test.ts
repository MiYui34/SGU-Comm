import { describe, expect, it } from "vitest";
import { createConnectionInfo } from "../src/lib/connectionInfo.svelte";
import type { Counters } from "../src/lib/connectionQuality";

const setup = () => {
  let counters: Counters = { good: 0, late: 0, lost: 0 };
  const info = createConnectionInfo("WebSocket", () => counters);
  return { info, setCounters: (c: Counters) => (counters = c) };
};

describe("createConnectionInfo", () => {
  it("measures the ping from our own timestamps only", () => {
    const { info } = setup();
    const { timestamp } = info.ping(1000);
    expect(timestamp).toBe(1_000_000); // µs
    info.pong(123_456, 1040); // not ours (the server's bridge pings too)
    expect(info.info.rtt).toBeNull();
    info.pong(timestamp, 1040);
    expect(info.info).toMatchObject({ level: "good", rtt: 40, spread: 0, transport: "WebSocket" });
    info.pong(timestamp, 1080); // a second answer to the same ping
    expect(info.info.rtt).toBe(40);
  });

  it("reports the stats in the next ping", () => {
    const { info, setCounters } = setup();
    info.pong(info.ping(0).timestamp, 30);
    info.pong(info.ping(5000).timestamp, 5050);
    setCounters({ good: 95, late: 4, lost: 1 });
    expect(info.ping(10_000).stats).toEqual({ good: 95, late: 4, lost: 1, tcpPackets: 2, tcpPingAvg: 40, tcpPingVar: 100 });
    expect(info.info).toMatchObject({ rtt: 40, spread: 10, bad: 5, late: 4, level: "fair" });
  });

  it("forgets what is older than 30 s", () => {
    const { info, setCounters } = setup();
    info.pong(info.ping(0).timestamp, 300);
    setCounters({ good: 0, late: 50, lost: 0 });
    info.ping(5000);
    info.pong(info.ping(31_000).timestamp, 31_020);
    expect(info.info).toMatchObject({ rtt: 20, bad: null, late: null, level: "good" });
  });
});
