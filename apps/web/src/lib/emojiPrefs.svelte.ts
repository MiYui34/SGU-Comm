import { addRecent, parseRecent } from "./emojiPicker";
import { persisted } from "./persisted.svelte";

// The emoji picker's choices, kept in the browser.
export const recentEmoji = persisted("emojiRecent", parseRecent);
export const skinTone = persisted("emojiTone", (raw) => {
  const n = Number(raw);
  return Number.isInteger(n) && n >= 0 && n <= 5 ? n : 0;
});

export const usedEmoji = (emoji: string) => recentEmoji.set(addRecent(recentEmoji.value, emoji));
