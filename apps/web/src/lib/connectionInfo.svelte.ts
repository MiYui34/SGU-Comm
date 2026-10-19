import type { PingStats } from "@kallisti/protocol";
import { badPercent, latePercent, meanVariance, qualityLevel, recent, type Counters, type Level } from "./connectionQuality";

export type ConnectionInfo = {
  level: Level | null;
  // Mean and spread of the ping in ms (null: none came back yet).
  rtt: number | null;
  spread: number | null;
  // Late or lost audio in percent (null: no audio came).
  bad: number | null;
  // Late only, in percent (see latePercent).
  late: number | null;
  transport: string;
};

// Pings and audio counters over the last 30 s, for the connection info and
// for the server (the stats go in the next ping). Times in ms on one clock
// (performance.now()).
export const createConnectionInfo = (transport: string, counters: () => Counters) => {
  let info = $state.raw<ConnectionInfo>({ level: null, rtt: null, spread: null, bad: null, late: null, transport });
  let rtts: { at: number; value: number }[] = [];
  let snapshots: (Counters & { at: number })[] = [];
  // Our pings on their way, by timestamp (µs): the server's bridge sends
  // pings of its own, and their answers come here too.
  const pending = new Set<number>();

  const refresh = () => {
    const stats = meanVariance(rtts.map((s) => s.value));
    const span = snapshots.length > 1 ? ([snapshots[0]!, snapshots.at(-1)!] as const) : null;
    const bad = span && badPercent(...span);
    const late = span && latePercent(...span);
    const rtt = stats?.mean ?? null;
    info = { level: qualityLevel(rtt, bad), rtt, spread: stats && Math.sqrt(stats.variance), bad, late, transport };
    return stats;
  };

  return {
    get info() {
      return info;
    },
    // The next ping's timestamp (µs) and stats. As in Mumble, tcpPackets
    // is the number of ping times behind the mean. good, late and lost are
    // UDP counters in Mumble; here the jitter buffer's totals since the
    // start, over all speakers.
    ping: (now: number): { timestamp: number; stats: PingStats } => {
      const current = counters();
      rtts = recent(rtts, now);
      snapshots = recent([...snapshots, { at: now, ...current }], now);
      const stats = refresh();
      const timestamp = Math.round(now * 1000);
      for (const t of pending) if (timestamp - t > 60e6) pending.delete(t);
      pending.add(timestamp);
      return {
        timestamp,
        stats: { ...current, tcpPackets: rtts.length, ...(stats && { tcpPingAvg: stats.mean, tcpPingVar: stats.variance }) },
      };
    },
    pong: (timestamp: number, now: number) => {
      if (!pending.delete(timestamp)) return;
      rtts = recent([...rtts, { at: now, value: now - timestamp / 1000 }], now);
      refresh();
    },
  };
};
