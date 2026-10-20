import type { TextChannel } from "./textChannels";

// Text channel categories (built-in server), one level like Discord's.
// mumble: the Mumble channel that holds its channels' mirrors (and its
// moderators).
export type TextCategory = { id: number; name: string; mumble?: number };
export type TextGroup = { category: TextCategory | null; channels: TextChannel[] };

// The sidebar's text channels: those without a (known) category first,
// then each category with its channels, in order.
export const textGroups = (channels: readonly TextChannel[], categories: readonly TextCategory[]): TextGroup[] => {
  const known = new Set(categories.map((c) => c.id));
  return [
    { category: null, channels: channels.filter((c) => c.category === undefined || !known.has(c.category)) },
    ...categories.map((category) => ({ category, channels: channels.filter((c) => c.category === category.id) })),
  ];
};

// The text channels in the sidebar's order.
export const textOrder = (channels: readonly TextChannel[], categories: readonly TextCategory[]): TextChannel[] =>
  textGroups(channels, categories).flatMap((g) => g.channels);

// A collapsed category still shows the open channel and unread ones, as
// in Discord.
export const shownCollapsed = (channels: readonly TextChannel[], current: number | null, unread: ReadonlySet<number>) =>
  channels.filter((c) => c.id === current || unread.has(c.id));

// Collapsed categories per server (category ids are the server's), on
// this device.
export type Collapsed = Readonly<Record<string, readonly number[]>>;

export const parseCollapsed = (raw: string | null): Collapsed => {
  let data: unknown;
  try {
    data = JSON.parse(raw ?? "null");
  } catch {
    return {};
  }
  if (!data || typeof data !== "object" || Array.isArray(data)) return {};
  return Object.fromEntries(
    Object.entries(data).flatMap(([server, v]) => {
      const ids = Array.isArray(v) ? v.filter((id) => Number.isInteger(id) && id >= 0) : [];
      return ids.length ? [[server, ids]] : [];
    }),
  );
};

export const collapsedIn = (all: Collapsed, server: string): ReadonlySet<number> => new Set(all[server] ?? []);

export const toggleCollapsed = (all: Collapsed, server: string, id: number): Collapsed => {
  const { [server]: before = [], ...rest } = all;
  const ids = before.includes(id) ? before.filter((x) => x !== id) : [...before, id];
  return ids.length ? { ...rest, [server]: ids } : rest;
};
