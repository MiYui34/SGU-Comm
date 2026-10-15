import type { ChannelNode, ServerState } from "@kallisti/protocol";
import type { TextChannel } from "./textChannels";

// The Mumble channels that mirror text channels for Mumble clients, and
// their parent: the web client shows them as text channels only.
export const mirrorIds = (server: ServerState, channels: readonly TextChannel[]): Set<number> => {
  const ids = new Set<number>();
  for (const c of channels) {
    if (c.mumble === undefined) continue;
    ids.add(c.mumble);
    const parent = server.channels[c.mumble]?.parent;
    if (parent !== null && parent !== undefined && parent !== 0) ids.add(parent);
  }
  return ids;
};

export const withoutMirrors = (node: ChannelNode | null, hidden: ReadonlySet<number>): ChannelNode | null =>
  node && {
    channel: node.channel,
    children: node.children.filter((c) => !hidden.has(c.channel.id)).map((c) => withoutMirrors(c, hidden)!),
  };
