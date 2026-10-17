// Per text channel, per server (channel ids are the server's): which
// messages notify, and a mute (until a time; 0: until turned off). Muted
// channels show no unread marker and notify only for mentions, as in
// Discord; "nothing" never notifies (badges still show).
export type NotifyLevel = "all" | "mentions" | "nothing";
export type ChannelPref = { level?: NotifyLevel; mutedUntil?: number };
export type ChannelPrefs = Readonly<Record<string, Readonly<Record<number, ChannelPref>>>>;

export const LEVELS: readonly [NotifyLevel, string][] = [
  ["all", "All messages"],
  ["mentions", "Only @mentions"],
  ["nothing", "Nothing"],
];
const DEFAULT_LEVEL: NotifyLevel = "mentions";
export const levelOf = (p: ChannelPref): NotifyLevel => p.level ?? DEFAULT_LEVEL;
const isLevel = (v: unknown): v is NotifyLevel => LEVELS.some(([l]) => l === v);

const parsePref = (v: unknown): ChannelPref | null => {
  if (!v || typeof v !== "object") return null;
  const { level, mutedUntil } = v as Record<string, unknown>;
  const pref: ChannelPref = {
    ...(isLevel(level) && level !== DEFAULT_LEVEL && { level }),
    ...(typeof mutedUntil === "number" && mutedUntil >= 0 && { mutedUntil }),
  };
  return Object.keys(pref).length ? pref : null;
};

// Also the old form: a list of muted channel ids per server.
export const parseChannelPrefs = (raw: string | null): ChannelPrefs => {
  let data: unknown;
  try {
    data = JSON.parse(raw ?? "null");
  } catch {
    return {};
  }
  if (!data || typeof data !== "object" || Array.isArray(data)) return {};
  const out: Record<string, Record<number, ChannelPref>> = {};
  for (const [server, v] of Object.entries(data)) {
    if (!v || typeof v !== "object") continue;
    const entries: [unknown, unknown][] = Array.isArray(v)
      ? v.filter((id) => Number.isInteger(id)).map((id) => [id, { mutedUntil: 0 }])
      : Object.entries(v);
    out[server] = Object.fromEntries(
      entries.flatMap(([id, p]) => {
        const pref = parsePref(p);
        return Number.isInteger(Number(id)) && pref ? [[Number(id), pref]] : [];
      }),
    );
  }
  return out;
};

export const prefOf = (prefs: ChannelPrefs, server: string, id: number): ChannelPref => prefs[server]?.[id] ?? {};

// patch: undefined removes a field; the default level is not stored.
export const setChannelPref = (prefs: ChannelPrefs, server: string, id: number, patch: ChannelPref): ChannelPrefs => {
  const next = Object.fromEntries(
    Object.entries({ ...prefOf(prefs, server, id), ...patch }).filter(([k, v]) => v !== undefined && !(k === "level" && v === DEFAULT_LEVEL)),
  ) as ChannelPref;
  const { [id]: _, ...rest } = prefs[server] ?? {};
  return { ...prefs, [server]: Object.keys(next).length ? { ...rest, [id]: next } : rest };
};

export const isMuted = (p: ChannelPref, now: number) => p.mutedUntil !== undefined && (p.mutedUntil === 0 || p.mutedUntil > now);

export const mutedIds = (prefs: ChannelPrefs, server: string, now: number): Set<number> =>
  new Set(
    Object.entries(prefs[server] ?? {})
      .filter(([, p]) => isMuted(p, now))
      .map(([id]) => Number(id)),
  );

export const withoutMuted = (unread: ReadonlySet<number>, muted: ReadonlySet<number>): Set<number> =>
  new Set([...unread].filter((id) => !muted.has(id)));

// Does a message notify (desktop notification, sound)? mentioned: it
// pings me.
export const notifies = (p: ChannelPref, mentioned: boolean, now: number): boolean => {
  if (p.level === "nothing") return false;
  if (mentioned) return true;
  return p.level === "all" && !isMuted(p, now);
};
