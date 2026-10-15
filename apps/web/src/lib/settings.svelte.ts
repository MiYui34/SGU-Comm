import { parseMuted, toggleMuted } from "./channelMute";
import { persisted } from "./persisted.svelte";
import { parseSettings, type Settings } from "./settings";
import { parseUserAudio, updateUserAudio, type UserAudio } from "./userAudio";

const voiceSettings = persisted("voiceSettings", parseSettings);

export const settings = {
  get value() {
    return voiceSettings.value;
  },
  update: (patch: Partial<Settings>) => voiceSettings.set({ ...voiceSettings.value, ...patch }),
};

const userAudioStore = persisted("userAudio", parseUserAudio);

export const userAudio = {
  get value() {
    return userAudioStore.value;
  },
  update: (name: string, patch: Partial<UserAudio>) =>
    userAudioStore.set(updateUserAudio(userAudioStore.value, name, patch)),
};

const mutedStore = persisted("mutedChannels", parseMuted);

export const mutedChannels = {
  get value() {
    return mutedStore.value;
  },
  toggle: (server: string, id: number) => mutedStore.set(toggleMuted(mutedStore.value, server, id)),
};
