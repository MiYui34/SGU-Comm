import type { ServerState } from "@kallisti/protocol";
import type { Person, VoiceRoom } from "./types";

// 根频道是没语音的大厅（id 0）：能进的房间都在它下面，而且自己不再带子
// 频道（带子频道的是分类，点进去没地方可坐）。
function topChannels(server: ServerState) {
  const parents = new Set<number>();
  for (const channel of Object.values(server.channels)) {
    if (channel.parent !== null) parents.add(channel.parent);
  }
  return Object.values(server.channels)
    .filter((channel) => !parents.has(channel.id))
    .sort((a, b) => a.position - b.position || a.name.localeCompare(b.name, "zh-CN"));
}

export function voiceRooms(server: ServerState): VoiceRoom[] {
  return topChannels(server)
    .filter((channel) => channel.id !== 0)
    .map((channel) => ({
      id: String(channel.id),
      name: channel.name,
      members: Object.values(server.users)
        .filter((user) => user.channelId === channel.id)
        .map((user) => user.name)
        .sort((a, b) => a.localeCompare(b, "zh-CN")),
    }));
}

// 大厅在服务器里叫 Root，界面上一律叫大厅。
function roomNameOf(server: ServerState, channelId: number): string {
  if (channelId === 0) return "大厅";
  return server.channels[channelId]?.name ?? "未知频道";
}

// 所有频道里的所有人，按频道顺序排：玩家页要的是「谁在哪儿」，不只是
// 谁在语音里，所以大厅里的人也在名单上。
export function voicePeople(server: ServerState): Person[] {
  const order = new Map<number, number>();
  // 大厅就是根频道，排在所有房间前面。
  order.set(0, -1);
  topChannels(server).forEach((channel, index) => order.set(channel.id, index));

  return Object.values(server.users)
    .map((user) => ({
      user,
      order: order.get(user.channelId) ?? Number.MAX_SAFE_INTEGER,
    }))
    .sort(
      (a, b) =>
        a.order - b.order ||
        roomNameOf(server, a.user.channelId).localeCompare(roomNameOf(server, b.user.channelId), "zh-CN") ||
        a.user.name.localeCompare(b.user.name, "zh-CN"),
    )
    .map(({ user }) => ({
      name: user.name,
      roomId: String(user.channelId),
      roomName: roomNameOf(server, user.channelId),
      // 服务器静音（mute）和自带静音（selfMute）都算关麦；被抑制（suppress）
      // 是管理员按住的，也说不了话。
      muted: user.mute || user.selfMute || user.suppress,
      deaf: user.deaf || user.selfDeaf,
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
