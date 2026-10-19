import { inVoice, type ServerState } from "@kallisti/protocol";

// Short sounds for voice: others joining or leaving our voice channel, our
// own join (also a move) and leave, mute and deafen.
export type Sound = "join" | "leave" | "selfJoin" | "selfLeave" | "mute" | "unmute" | "deafen" | "undeafen";

export type VoiceSnapshot = {
  me: number;
  voice: number | null; // our voice channel (null: not in voice)
  users: Readonly<Record<number, { channelId: number }>>;
  selfMute: boolean;
  selfDeaf: boolean;
};

export const snapshotOf = (state: ServerState): VoiceSnapshot | null => {
  const me = state.session === null ? undefined : state.users[state.session];
  if (!me) return null;
  const { session, channelId, selfMute, selfDeaf } = me;
  return { me: session, voice: inVoice(state) ? channelId : null, users: state.users, selfMute, selfDeaf };
};

// At most one sound per change. None for the first snapshot (the sync at
// login or after a reconnect), and none while deafened but the undeafen.
export const voiceSound = (prev: VoiceSnapshot | null, next: VoiceSnapshot): Sound | null => {
  if (!prev || prev.me !== next.me) return null;
  if (prev.selfDeaf !== next.selfDeaf) return next.selfDeaf ? "deafen" : "undeafen";
  if (next.selfDeaf) return null;
  if (prev.voice !== next.voice) return next.voice === null ? "selfLeave" : "selfJoin";
  if (prev.selfMute !== next.selfMute) return next.selfMute ? "mute" : "unmute";
  if (next.voice === null) return null;
  const inside = (s: VoiceSnapshot) =>
    new Set(Object.entries(s.users).flatMap(([id, u]) => (u.channelId === next.voice ? [id] : [])));
  const [before, after] = [inside(prev), inside(next)];
  if ([...after].some((s) => !before.has(s))) return "join";
  if ([...before].some((s) => !after.has(s))) return "leave";
  return null;
};

// Many changes at once (a server restart, a busy channel): one sound.
export const SOUND_GAP_MS = 500;
export const allowSound = (last: number, now: number) => now - last >= SOUND_GAP_MS;
