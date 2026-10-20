import { isBlocked, parseBlocked, toggleBlocked } from "./blocked";
import { collapsedIn, parseCollapsed, toggleCollapsed } from "./categories";
import { parseChannelPrefs, setChannelPref, type ChannelPref } from "./channelPrefs";
import { directPrefsOf, parseDirectPrefs, setDirectPrefs, type DirectPrefs } from "./directList";
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

// The key from when only mutes were kept (parseChannelPrefs reads both).
const channelStore = persisted("mutedChannels", parseChannelPrefs);

export const channelPrefs = {
  get value() {
    return channelStore.value;
  },
  set: (server: string, id: number, patch: ChannelPref) => channelStore.set(setChannelPref(channelStore.value, server, id, patch)),
};

// Collapsed text categories, per server (see lib/categories).
const collapsedStore = persisted("collapsedCategories", parseCollapsed);

export const collapsedCategories = {
  of: (server: string) => collapsedIn(collapsedStore.value, server),
  toggle: (server: string, id: number) => collapsedStore.set(toggleCollapsed(collapsedStore.value, server, id)),
};

const directStore = persisted("directChats", parseDirectPrefs);

export const directPrefs = {
  of: (server: string) => directPrefsOf(directStore.value, server),
  update: (server: string, fn: (p: DirectPrefs) => DirectPrefs) => {
    const before = directPrefsOf(directStore.value, server);
    const next = fn(before);
    if (next !== before) directStore.set(setDirectPrefs(directStore.value, server, next));
  },
};

const blockedStore = persisted("blockedUsers", parseBlocked);

export const blockedUsers = {
  get list() {
    return blockedStore.value;
  },
  has: (name: string | null) => isBlocked(blockedStore.value, name),
  toggle: (name: string) => blockedStore.set(toggleBlocked(blockedStore.value, name)),
};
