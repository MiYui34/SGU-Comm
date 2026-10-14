import { parseSettings, type Settings } from "./settings";

const STORAGE_KEY = "voiceSettings";

let current = $state<Settings>(parseSettings(localStorage.getItem(STORAGE_KEY)));

export const settings = {
  get value(): Settings {
    return current;
  },
  update: (patch: Partial<Settings>) => {
    current = { ...current, ...patch };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(current));
  },
};
