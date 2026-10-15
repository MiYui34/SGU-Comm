export type SelfState = { selfMute: boolean; selfDeaf: boolean };

// Same rules as the Mumble client: deaf implies mute, and unmuting while
// deaf also undeafens.
export const toggleMute = ({ selfMute, selfDeaf }: SelfState): SelfState =>
  selfMute ? { selfMute: false, selfDeaf: false } : { selfMute: true, selfDeaf };

export const toggleDeaf = ({ selfDeaf }: SelfState): SelfState =>
  selfDeaf ? { selfMute: false, selfDeaf: false } : { selfMute: true, selfDeaf: true };

// The microphone is open (the browser shows it recording) only while it is
// used: in a voice channel and not muted, or in the voice settings (level
// meter).
export const micWanted = (s: { inVoice: boolean; selfMute: boolean; settingsOpen: boolean }): boolean =>
  (s.inVoice && !s.selfMute) || s.settingsOpen;
