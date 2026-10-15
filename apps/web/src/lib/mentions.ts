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
