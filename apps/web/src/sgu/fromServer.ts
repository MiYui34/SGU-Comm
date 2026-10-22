import type { ServerState } from "@kallisti/protocol";
import type { VoiceRoom } from "./types";

// 内置服务器的根频道是没语音的大厅：能进的房间都在它下面，而且自己不再
// 带子频道（带子频道的是分类，点进去没地方可坐）。
export function voiceRooms(server: ServerState): VoiceRoom[] {
  const parents = new Set<number>();
  for (const channel of Object.values(server.channels)) {
    if (channel.parent !== null) parents.add(channel.parent);
  }
  return Object.values(server.channels)
    .filter((channel) => channel.id !== 0 && !parents.has(channel.id))
    .sort((a, b) => a.position - b.position || a.name.localeCompare(b.name, "zh-CN"))
    .map((channel) => ({
      id: String(channel.id),
      name: channel.name,
      members: Object.values(server.users)
        .filter((user) => user.channelId === channel.id)
        .map((user) => user.name)
        .sort((a, b) => a.localeCompare(b, "zh-CN")),
    }));
}

// 自己的名字：还没同步到（或在游客登录前）就是 null。
export function myNameOf(server: ServerState): string | null {
  const session = server.session;
  if (session === null) return null;
  return server.users[session]?.name ?? null;
}

// 自己在哪个房间（房间号是字符串，和 VoiceRoom.id 一致）；不在任何房间里
// 则返回 "0" —— 根频道就是大厅。
export function myChannelOf(server: ServerState): string | null {
  const session = server.session;
  if (session === null) return null;
  const channel = server.users[session];
  return channel === undefined ? null : String(channel.channelId);
}
