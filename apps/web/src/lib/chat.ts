import type { ChatMessage } from "@kallisti/protocol";

export type MessageGroup = {
  actor: number | null;
  actorName: string | null;
  at: number;
  messages: ChatMessage[];
};

const GROUP_WINDOW_MS = 5 * 60_000;

const sameDay = (a: number, b: number) => new Date(a).toDateString() === new Date(b).toDateString();

// Discord-style grouping: one header for a run of messages from one sender,
// within 5 minutes and on the same day.
export const groupMessages = (messages: readonly ChatMessage[]): MessageGroup[] => {
  const groups: MessageGroup[] = [];
  for (const m of messages) {
    const last = groups.at(-1);
    const prev = last?.messages.at(-1);
    const sameAuthor = last?.actor === m.actor && last.actorName === m.actorName;
    if (last && prev && sameAuthor && m.at - prev.at <= GROUP_WINDOW_MS && sameDay(prev.at, m.at)) {
      last.messages.push(m);
    } else {
      groups.push({ actor: m.actor, actorName: m.actorName, at: m.at, messages: [m] });
    }
  }
  return groups;
};

export type ChatItem = { kind: "day"; at: number } | { kind: "group"; group: MessageGroup };

// Groups with a date divider before the first group of each day.
export const chatItems = (messages: readonly ChatMessage[]): ChatItem[] =>
  groupMessages(messages).flatMap((group, i, all) => {
    const prev = all[i - 1];
    const item: ChatItem = { kind: "group", group };
    return prev && sameDay(prev.at, group.at) ? [item] : [{ kind: "day", at: group.at }, item];
  });

export const formatTime = (at: number, locale?: string) =>
  new Date(at).toLocaleTimeString(locale, { hour: "numeric", minute: "2-digit" });

export const formatDay = (at: number, locale?: string) =>
  new Date(at).toLocaleDateString(locale, { year: "numeric", month: "long", day: "numeric" });

// "Today at 3:58 PM", "Yesterday at 9:05 AM", else "10/1/2026 9:05 AM".
export const formatTimestamp = (at: number, now: number, locale?: string) => {
  const time = formatTime(at, locale);
  if (sameDay(at, now)) return `Today at ${time}`;
  if (sameDay(at, now - 86_400_000)) return `Yesterday at ${time}`;
  return `${new Date(at).toLocaleDateString(locale)} ${time}`;
};

// Unread private messages per sender name: received minus read.
export const unreadCounts = (
  counts: Readonly<Record<string, number>>,
  read: Readonly<Record<string, number>>,
): Record<string, number> =>
  Object.fromEntries(
    Object.entries(counts)
      .map(([name, n]) => [name, n - (read[name] ?? 0)] as const)
      .filter(([, n]) => n > 0),
  );
