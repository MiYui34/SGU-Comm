import type { AudioFormat } from "./audio";
import type { Channel, ChatMessage, ServerState, User } from "./state";

export type ChannelNode = { channel: Channel; children: ChannelNode[] };

const byName = (a: string, b: string) => a.localeCompare(b, undefined, { sensitivity: "base" });

const byPositionThenName = (a: Channel, b: Channel) =>
  a.position - b.position || byName(a.name, b.name);

export const channelTree = (state: ServerState): ChannelNode | null => {
  const root = state.channels[0];
  if (!root) return null;
  const byParent = Map.groupBy(Object.values(state.channels), (c) => c.parent);
  // Each channel has one parent, so the only cycle reachable from root goes
  // through root itself. Skipping root as a child is enough.
  const build = (channel: Channel): ChannelNode => ({
    channel,
    children: (byParent.get(channel.id) ?? [])
      .filter((c) => c.id !== root.id)
      .sort(byPositionThenName)
      .map(build),
  });
  return build(root);
};

// A channel and the others with the same parent, in tree order. Root has
// no siblings.
export const channelSiblings = (state: ServerState, channelId: number): Channel[] => {
  const parent = state.channels[channelId]?.parent;
  if (channelId === 0 || parent === null || parent === undefined) return [];
  return Object.values(state.channels)
    .filter((c) => c.parent === parent && c.id !== 0)
    .sort(byPositionThenName);
};

export const usersSorted = (state: ServerState): User[] =>
  Object.values(state.users).sort((a, b) => byName(a.name, b.name));

export const usersIn = (state: ServerState, channelId: number): User[] =>
  usersSorted(state).filter((u) => u.channelId === channelId);

const ancestorsOf = (state: ServerState, channelId: number): Set<number> => {
  const ids = new Set<number>();
  for (let id: number | null = channelId; id !== null && !ids.has(id); ) {
    ids.add(id);
    id = state.channels[id]?.parent ?? null;
  }
  return ids;
};

// Messages sent to the channel itself, or to a channel tree that contains it.
export const messagesIn = (state: ServerState, channelId: number): ChatMessage[] => {
  const trees = ancestorsOf(state, channelId);
  return state.messages.filter(
    (m) => m.channelIds.includes(channelId) || m.treeIds.some((id) => trees.has(id)),
  );
};

export const audioFormat = ({ serverVersion: v }: ServerState): AudioFormat =>
  v && (v.major > 1 || (v.major === 1 && v.minor >= 5)) ? "protobuf" : "legacy";

// The private conversation with one person, by name.
export const privateMessagesWith = (state: ServerState, peer: string): ChatMessage[] =>
  state.messages.filter((m) => m.peer === peer);

// Channels whose long description is known only by its hash.
export const missingDescriptions = (state: ServerState): number[] =>
  Object.values(state.channels)
    .filter((c) => c.descriptionHash !== null && c.description === "")
    .map((c) => c.id);

// Users whose avatar is known only by its hash.
export const missingTextures = (state: ServerState): number[] =>
  Object.values(state.users)
    .filter((u) => u.textureHash !== null && u.texture === null)
    .map((u) => u.session);

// Mumble's permission bits (the ones the client looks at).
export const Perm = {
  Write: 0x1,
  Enter: 0x4,
  Speak: 0x8,
  MakeChannel: 0x40,
  MakeTempChannel: 0x400,
} as const;

const SPEAK = Perm.Speak;

// Whether we may do something in a channel, from the server's answers
// (false while unknown; ask with queryPermissions).
export const may = (state: ServerState, channelId: number, perm: number): boolean => {
  const known = state.channelPermissions[channelId] ?? (channelId === 0 ? state.rootPermissions : null);
  return known !== null && known !== undefined && (known & perm) === perm;
};

// Kallisti servers keep the root channel as a lobby without voice: users
// arrive there and join voice channels on purpose. Plain Mumble servers
// use root as a voice channel.
export const isLobby = (state: ServerState): boolean =>
  state.rootPermissions !== null && (state.rootPermissions & SPEAK) === 0;

export const inVoice = (state: ServerState): boolean => {
  const me = state.session === null ? undefined : state.users[state.session];
  return me !== undefined && !(isLobby(state) && me.channelId === 0);
};

// A ban's address for people: IPv4 (::ffff:a.b.c.d) as a.b.c.d/n.
export const banAddress = ({ address: a, mask }: { address: Uint8Array; mask: number }): string => {
  const v4 = a.length === 16 && a.subarray(0, 10).every((x) => x === 0) && a[10] === 0xff && a[11] === 0xff;
  if (v4) return `${a[12]}.${a[13]}.${a[14]}.${a[15]}/${mask - 96}`;
  const groups = [];
  for (let i = 0; i < a.length; i += 2) groups.push(((a[i]! << 8) | a[i + 1]!).toString(16));
  return `${groups.join(":")}/${mask}`;
};
