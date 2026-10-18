import { qualityLevel, type Level } from "./connectionQuality";

// The admin view of all connections (Server settings → Connections): what
// the server measured over the last 30 to 40 s. No addresses: admins see
// them in Mumble's user information, the table need not show them around.
export type Connection = {
  session: number;
  name: string;
  transport: string; // of its voice: "udp", "tcp" or "websocket"
  ping: number | null; // ms, as the client measured it (null: none yet)
  // Late or lost voice in percent: what the server got from the client,
  // and what the client says it got (null: too little voice).
  fromClient: number | null;
  fromServer: number | null;
  dropped: number; // voice frames it did not get: its queue was full
  overflows: number; // text messages it lost
  online: number; // seconds
  client: string;
  os: string;
};

// The admin view asks again this often while it is open.
export const POLL_MS = 5_000;

const isNumOrNull = (v: unknown) => v === null || (typeof v === "number" && Number.isFinite(v));
const isConnection = (v: unknown): v is Connection => {
  if (!v || typeof v !== "object") return false;
  const c = v as Record<string, unknown>;
  return (
    ["session", "dropped", "overflows", "online"].every((k) => typeof c[k] === "number") &&
    ["name", "transport", "client", "os"].every((k) => typeof c[k] === "string") &&
    ["ping", "fromClient", "fromServer"].every((k) => isNumOrNull(c[k]))
  );
};

export const parseConnections = (v: unknown): Connection[] | null => (Array.isArray(v) ? v.filter(isConnection) : null);

// As the own connection info, by what is known: the worse direction counts.
export const connectionLevel = (c: Connection): Level | null => {
  const bad = [c.fromClient, c.fromServer].filter((p) => p !== null);
  if (c.ping === null && !bad.length) return null;
  return qualityLevel(c.ping ?? 0, bad.length ? Math.max(...bad) : null);
};

const RANK = { poor: 0, fair: 1, good: 2, unknown: 3 };

// Worst first: by level (not known yet last), then the slower ping.
export const worstFirst = (list: readonly Connection[]): Connection[] =>
  list
    .map((c) => ({ c, rank: RANK[connectionLevel(c) ?? "unknown"] }))
    .sort((a, b) => a.rank - b.rank || (b.c.ping ?? -1) - (a.c.ping ?? -1))
    .map(({ c }) => c);

export const formatPing = (ms: number | null) => (ms === null ? "–" : `${Math.round(ms)} ms`);

export const formatPercent = (p: number | null) => (p === null ? "–" : `${p < 10 ? Math.round(p * 10) / 10 : Math.round(p)} %`);

export const formatOnline = (s: number) => {
  const [d, h, m] = [Math.floor(s / 86400), Math.floor(s / 3600) % 24, Math.floor(s / 60) % 60];
  if (d) return `${d} d ${h} h`;
  if (h) return `${h} h ${m} min`;
  return m ? `${m} min` : `${s} s`;
};
