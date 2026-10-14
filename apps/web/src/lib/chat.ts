import type { ChatMessage } from "@mumble-discord/protocol";

export type MessageGroup = {
  actor: number | null;
  actorName: string | null;
  at: number;
  messages: ChatMessage[];
};

const GROUP_WINDOW_MS = 5 * 60_000;

// Discord-style grouping: one header for a run of messages from one sender.
export const groupMessages = (messages: readonly ChatMessage[]): MessageGroup[] =>
  messages.reduce<MessageGroup[]>((groups, m) => {
    const last = groups.at(-1);
    const prev = last?.messages.at(-1);
    if (last && prev && last.actor === m.actor && m.at - prev.at <= GROUP_WINDOW_MS) {
      return [...groups.slice(0, -1), { ...last, messages: [...last.messages, m] }];
    }
    return [...groups, { actor: m.actor, actorName: m.actorName, at: m.at, messages: [m] }];
  }, []);

export const formatTime = (at: number) =>
  new Date(at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
