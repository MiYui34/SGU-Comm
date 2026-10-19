import type { ServerState } from "@kallisti/protocol";
import { playNotes } from "./ping";

// Short sounds for our own mute and deafen (join and leave sounds were
// tried and dropped: too much noise).
export type Sound = "mute" | "unmute" | "deafen" | "undeafen";

export type VoiceSnapshot = { me: number; selfMute: boolean; selfDeaf: boolean };

export const snapshotOf = (state: ServerState): VoiceSnapshot | null => {
  const me = state.session === null ? undefined : state.users[state.session];
  if (!me) return null;
  return { me: me.session, selfMute: me.selfMute, selfDeaf: me.selfDeaf };
};

// At most one sound per change. None for the first snapshot (the sync at
// login or after a reconnect), and none while deafened but the undeafen.
export const voiceSound = (prev: VoiceSnapshot | null, next: VoiceSnapshot): Sound | null => {
  if (!prev || prev.me !== next.me) return null;
  if (prev.selfDeaf !== next.selfDeaf) return next.selfDeaf ? "deafen" : "undeafen";
  if (next.selfDeaf) return null;
  if (prev.selfMute !== next.selfMute) return next.selfMute ? "mute" : "unmute";
  return null;
};

// Many changes at once (a server restart, a busy channel): one sound.
export const SOUND_GAP_MS = 500;
export const allowSound = (last: number, now: number) => now - last >= SOUND_GAP_MS;

// Our own notes (Hz) and the time between them (s): rising for unmuting,
// falling for muting, lower ones for deafening.
const NOTES: Record<Sound, [readonly number[], number]> = {
  mute: [[660, 440], 0.05],
  unmute: [[440, 660], 0.05],
  deafen: [[392, 262], 0.07],
  undeafen: [[262, 392], 0.07],
};

export const playSound = (sound: Sound, outputDeviceId: string | null, volume: number) => {
  const [notes, step] = NOTES[sound];
  playNotes(outputDeviceId, notes, step, 0.3 * volume);
};
