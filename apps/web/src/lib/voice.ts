export type SelfState = { selfMute: boolean; selfDeaf: boolean };

// Same rules as the Mumble client: deaf implies mute, and unmuting while
// deaf also undeafens.
export const toggleMute = ({ selfMute, selfDeaf }: SelfState): SelfState =>
  selfMute ? { selfMute: false, selfDeaf: false } : { selfMute: true, selfDeaf };

export const toggleDeaf = ({ selfDeaf }: SelfState): SelfState =>
  selfDeaf ? { selfMute: false, selfDeaf: false } : { selfMute: true, selfDeaf: true };

// The microphone is open (the browser shows it recording) only while it is
// used: in a voice channel, or in the voice settings (level meter). Muted it
// stays open (nothing is sent): opening a microphone makes the browser
// reconfigure the audio device, which interrupts the sound. closeWhenMuted
// closes it anyway.
export const micWanted = (s: {
  inVoice: boolean;
  selfMute: boolean;
  settingsOpen: boolean;
  closeWhenMuted: boolean;
}): boolean => (s.inVoice && !(s.selfMute && s.closeWhenMuted)) || s.settingsOpen;
