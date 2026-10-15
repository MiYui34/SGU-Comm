// Emoji reactions on stored messages (the server keeps them).
export type Reaction = { emoji: string; users: string[] };

// The picker's choice, in the server's form (sidecar normalEmoji: older
// symbols with the emoji variation selector).
export const COMMON_EMOJI = ["👍", "❤\uFE0F", "😂", "🎉", "😮", "😢", "👀", "🙏", "🔥", "✅\uFE0F"];

// Names compare without case, as the server's names do.
export const reactedBy = (r: Reaction, me: string) => r.users.some((u) => u.toLowerCase() === me.toLowerCase());

export const reactRequest = (message: number, emoji: string, on: boolean) =>
  JSON.stringify({ type: "react", message, emoji, on });
