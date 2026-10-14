import type { Channel, ServerState, User } from "./state";

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

export const usersIn = (state: ServerState, channelId: number): User[] =>
  Object.values(state.users)
    .filter((u) => u.channelId === channelId)
    .sort((a, b) => byName(a.name, b.name));
