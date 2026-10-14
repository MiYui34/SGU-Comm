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

// The private conversation between us and one other session.
export const privateMessagesWith = (state: ServerState, other: number): ChatMessage[] =>
  state.messages.filter(
    (m) =>
      m.channelIds.length === 0 &&
      ((m.actor === other && m.sessions.includes(state.session ?? -1)) ||
        (m.actor === state.session && m.sessions.includes(other))),
  );

// Channels whose long description is known only by its hash.
export const missingDescriptions = (state: ServerState): number[] =>
  Object.values(state.channels)
    .filter((c) => c.descriptionHash !== null && c.description === "")
    .map((c) => c.id);
