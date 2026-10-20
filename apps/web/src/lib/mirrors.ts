import type { ChannelNode, ServerState } from "@kallisti/protocol";
import type { TextCategory } from "./categories";
import type { TextChannel } from "./textChannels";

// The Mumble channels that mirror text channels for Mumble clients, the
// categories that hold them, and their parent: the web client shows them
// as text channels only.
export const mirrorIds = (server: ServerState, channels: readonly TextChannel[], categories: readonly TextCategory[] = []): Set<number> => {
  const ids = new Set<number>();
  for (const c of [...channels, ...categories]) {
    // The mirror or category and those above it, up to root.
    for (let id = c.mumble; id !== undefined && id !== 0 && !ids.has(id); id = server.channels[id]?.parent ?? undefined) ids.add(id);
  }
  return ids;
};

export const withoutMirrors = (node: ChannelNode | null, hidden: ReadonlySet<number>): ChannelNode | null =>
  node && {
    channel: node.channel,
    children: node.children.filter((c) => !hidden.has(c.channel.id)).map((c) => withoutMirrors(c, hidden)!),
  };
