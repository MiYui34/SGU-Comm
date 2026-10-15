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
