import { escapeHtml } from "@kallisti/protocol";
import type { TextState } from "./textChannels";

// Mentions: "@" plus a user's whole name, any case, after a space or an
// opening mark (not in an address, a URL or right after an entity).
// Messages here are escaped HTML, so the name is matched escaped too; tags
// are skipped.
const NAME_CHAR = "[\\p{L}\\p{N}_-]";
let cached: { name: string; re: RegExp } | null = null;
const pattern = (name: string) => {
  if (cached?.name !== name) {
    const escaped = escapeHtml(name).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    cached = { name, re: new RegExp(`(?<=^|[\\s(\\[{,:])@${escaped}(?!${NAME_CHAR})`, "giu") };
  }
  cached.re.lastIndex = 0;
  return cached.re;
};

// Text outside tags, then tags, alternating.
const parts = (html: string) => html.split(/(<[^>]*>)/);

export const mentions = (html: string, name: string): boolean =>
  name !== "" && parts(html).some((p, i) => i % 2 === 0 && pattern(name).test(p));

const sameName = (a: string, b: string) => a.toLowerCase() === b.toLowerCase();

// A message that mentions me, not written by me (author null: the server).
export const mentionsMe = (html: string, author: string | null, me: string): boolean =>
  (author === null || !sameName(author, me)) && mentions(html, me);

export const highlightMentions = (html: string, name: string): string =>
  name === ""
    ? html
    : parts(html)
        .map((p, i) => (i % 2 === 0 ? p.replace(pattern(name), (m) => `<span class="mention">${m}</span>`) : p))
        .join("");

// Unseen messages per channel that mention me (not my own), for badges.
// Messages are sorted by id: count from the newest down to the last seen.
export const mentionCounts = (state: TextState, seen: Record<number, number>, me: string): Record<number, number> => {
  const out: Record<number, number> = {};
  for (const [id, msgs] of Object.entries(state.messages)) {
    const channel = Number(id);
    let n = 0;
    for (let i = msgs.length - 1; i >= 0 && msgs[i]!.id > (seen[channel] ?? 0); i--) {
      if (mentionsMe(escapeHtml(msgs[i]!.text), msgs[i]!.author, me)) n++;
    }
    if (n) out[channel] = n;
  }
  return out;
};

// Autocomplete: the "@word" being typed before the caret (null: none).
export const mentionQuery = (text: string, caret: number): { start: number; query: string } | null => {
  const m = /(?:^|[\s(\[{,:])@([^\s@]*)$/u.exec(text.slice(0, caret));
  return m ? { start: caret - m[1]!.length - 1, query: m[1]! } : null;
};

const SUGGESTIONS = 8;
const byName = (a: string, b: string) => a.localeCompare(b, undefined, { sensitivity: "base" });

// Names to offer: those starting with the query first, then those that
// contain it; never me, each once.
export const suggestNames = (names: readonly string[], query: string, me: string): string[] => {
  const q = query.toLowerCase();
  const unique = [...new Map(names.map((n) => [n.toLowerCase(), n])).entries()].filter(([low]) => low !== me.toLowerCase());
  const starts = unique.filter(([low]) => low.startsWith(q)).map(([, n]) => n).sort(byName);
  const contains = unique.filter(([low]) => !low.startsWith(q) && low.includes(q)).map(([, n]) => n).sort(byName);
  return [...starts, ...contains].slice(0, SUGGESTIONS);
};

// Puts "@name " in place of the typed "@word"; the caret goes after the
// space (a space already there is kept, not doubled).
export const insertMention = (text: string, start: number, caret: number, name: string) => {
  const rest = text.slice(caret);
  const mention = /^\s/u.test(rest) ? `@${name}` : `@${name} `;
  return { text: text.slice(0, start) + mention + rest, caret: start + name.length + 2 };
};
