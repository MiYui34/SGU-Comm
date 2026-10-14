export type SelfState = { selfMute: boolean; selfDeaf: boolean };

// Same rules as the Mumble client: deaf implies mute, and unmuting while
// deaf also undeafens.
export const toggleMute = ({ selfMute, selfDeaf }: SelfState): SelfState =>
  selfMute ? { selfMute: false, selfDeaf: false } : { selfMute: true, selfDeaf };

export const toggleDeaf = ({ selfDeaf }: SelfState): SelfState =>
  selfDeaf ? { selfMute: false, selfDeaf: false } : { selfMute: true, selfDeaf: true };
