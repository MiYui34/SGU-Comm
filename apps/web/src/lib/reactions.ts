// Emoji reactions on stored messages (the server keeps them).
// users: names to show; ids: their Mumble user ids, in the same order.
export type Reaction = { emoji: string; users: string[]; ids: number[] };

// The picker's choice, in the server's form (sidecar normalEmoji: older
// symbols with the emoji variation selector).
export const COMMON_EMOJI = ["👍", "❤\uFE0F", "😂", "🎉", "😮", "😢", "👀", "🙏", "🔥", "✅\uFE0F"];

// Ours: by our user id (null: not registered, so none).
export const reactedBy = (r: Reaction, myId: number | null) => myId !== null && r.ids.includes(myId);

export const reactRequest = (message: number, emoji: string, on: boolean) =>
  JSON.stringify({ type: "react", message, emoji, on });
