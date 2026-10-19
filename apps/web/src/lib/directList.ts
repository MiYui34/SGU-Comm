// Direct chats in the sidebar, per server (chat ids are the server's), kept
// on this device like channel settings: closed ones (by the newest message
// id when closed: a newer message shows the chat again) and pinned ones (in
// pin order, above the others).
export type DirectPrefs = { closed: Readonly<Record<number, number>>; pinned: readonly number[] };
export type AllDirectPrefs = Readonly<Record<string, DirectPrefs>>;

const EMPTY: DirectPrefs = { closed: {}, pinned: [] };
const isId = (v: unknown): v is number => Number.isInteger(v) && (v as number) >= 0;

const parseOne = (v: unknown): DirectPrefs => {
  const { closed, pinned } = (v && typeof v === "object" ? v : {}) as Record<string, unknown>;
  return {
    closed: Object.fromEntries(
      Object.entries(closed && typeof closed === "object" ? closed : {}).filter(([id, n]) => isId(Number(id)) && isId(n)),
    ),
    pinned: Array.isArray(pinned) ? [...new Set(pinned.filter(isId))] : [],
  };
};

export const parseDirectPrefs = (raw: string | null): AllDirectPrefs => {
  let data: unknown;
  try {
    data = JSON.parse(raw ?? "null");
  } catch {
    return {};
  }
  if (!data || typeof data !== "object" || Array.isArray(data)) return {};
  return Object.fromEntries(
    Object.entries(data).flatMap(([server, v]) => (v && typeof v === "object" ? [[server, parseOne(v)]] : [])),
  );
};

export const directPrefsOf = (all: AllDirectPrefs, server: string): DirectPrefs => all[server] ?? EMPTY;

export const setDirectPrefs = (all: AllDirectPrefs, server: string, p: DirectPrefs): AllDirectPrefs => {
  const { [server]: _, ...rest } = all;
  return Object.keys(p.closed).length || p.pinned.length ? { ...rest, [server]: p } : rest;
};

export const closeDirect = (p: DirectPrefs, channel: number, latest: number): DirectPrefs => ({
  ...p,
  closed: { ...p.closed, [channel]: latest },
});

// Opened again (from a user menu, a link): back in the list.
export const reopenDirect = (p: DirectPrefs, channel: number): DirectPrefs => {
  if (!(channel in p.closed)) return p;
  const { [channel]: _, ...closed } = p.closed;
  return { ...p, closed };
};

export const togglePin = (p: DirectPrefs, channel: number): DirectPrefs => ({
  ...p,
  pinned: p.pinned.includes(channel) ? p.pinned.filter((id) => id !== channel) : [...p.pinned, channel],
});

// latest: the newest message id in the chat.
export const visibleDirects = <D extends { channel: number; latest: number }>(list: readonly D[], p: DirectPrefs): D[] => {
  const pin = (d: D) => {
    const i = p.pinned.indexOf(d.channel);
    return i < 0 ? Infinity : i;
  };
  return list
    .filter((d) => !(d.channel in p.closed) || d.latest > p.closed[d.channel]!)
    .toSorted((a, b) => pin(a) - pin(b) || b.latest - a.latest);
};

// Muted direct chats count no unread messages (every message there is for
// us, so a mute hides them all; muted channels keep their mentions).
export const quietDirects = (
  counts: Readonly<Record<number, number>>,
  directs: Readonly<Record<number, unknown>>,
  muted: ReadonlySet<number>,
): Record<number, number> => Object.fromEntries(Object.entries(counts).filter(([id]) => !(id in directs && muted.has(Number(id)))));
