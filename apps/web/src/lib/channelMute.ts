// Muted text channels, per server (channel ids are the server's): no
// unread marker. Mentions still count and notify.
export type MutedPrefs = Readonly<Record<string, readonly number[]>>;

export const parseMuted = (raw: string | null): MutedPrefs => {
  let data: unknown;
  try {
    data = JSON.parse(raw ?? "null");
  } catch {
    return {};
  }
  if (!data || typeof data !== "object" || Array.isArray(data)) return {};
  const out: Record<string, number[]> = {};
  for (const [server, ids] of Object.entries(data)) {
    if (Array.isArray(ids)) out[server] = ids.filter((id): id is number => Number.isInteger(id));
  }
  return out;
};

export const mutedOn = (prefs: MutedPrefs, server: string): ReadonlySet<number> => new Set(prefs[server] ?? []);

export const toggleMuted = (prefs: MutedPrefs, server: string, id: number): MutedPrefs => {
  const ids = prefs[server] ?? [];
  return { ...prefs, [server]: ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id] };
};

export const withoutMuted = (unread: ReadonlySet<number>, muted: ReadonlySet<number>): Set<number> =>
  new Set([...unread].filter((id) => !muted.has(id)));
