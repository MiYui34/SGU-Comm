import { inVoice, type ServerState } from "@kallisti/protocol";
import { playNotes } from "./ping";

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
  if (next.voice === null || prev.users === next.users) return null;
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

// Our own notes (Hz) and the time between them (s): rising for joins and
// unmuting, falling for leaves and muting; three notes for our own join
// and leave, lower ones for deafening.
const NOTES: Record<Sound, [readonly number[], number]> = {
  join: [[523, 784], 0.09],
  leave: [[784, 523], 0.09],
  selfJoin: [[523, 659, 784], 0.08],
  selfLeave: [[784, 659, 523], 0.08],
  mute: [[660, 440], 0.05],
  unmute: [[440, 660], 0.05],
  deafen: [[392, 262], 0.07],
  undeafen: [[262, 392], 0.07],
};

export const playSound = (sound: Sound, outputDeviceId: string | null, volume: number) => {
  const [notes, step] = NOTES[sound];
  playNotes(outputDeviceId, notes, step, 0.3 * volume);
};
