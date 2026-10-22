import type { VoiceRoom } from "./types";

/**
 * 把自己移进某个房间：先把自己从所有房间摘掉，再放到目标房间末尾。
 * 重复点同一个房间就是留在原地（不会出现两个自己）。
 */
export function joinRoom(rooms: VoiceRoom[], me: string, id: string): VoiceRoom[] {
  return rooms.map((room) => {
    const others = room.members.filter((name) => name !== me);
    return { ...room, members: room.id === id ? [...others, me] : others };
  });
}

/** 离开语音：把自己从所有房间里摘掉。 */
export function leaveRooms(rooms: VoiceRoom[], me: string): VoiceRoom[] {
  return rooms.map((room) => ({
    ...room,
    members: room.members.filter((name) => name !== me),
  }));
}
