// Emoji reactions on stored messages (the server keeps them).
export type Reaction = { emoji: string; users: string[] };

// The picker's choice; any emoji from elsewhere works too.
export const COMMON_EMOJI = ["👍", "❤️", "😂", "🎉", "😮", "😢", "👀", "🙏", "🔥", "✅"];

// Names compare without case, as the server's names do.
export const reactedBy = (r: Reaction, me: string) => r.users.some((u) => u.toLowerCase() === me.toLowerCase());

export const reactRequest = (message: number, emoji: string, on: boolean) =>
  JSON.stringify({ type: "react", message, emoji, on });
