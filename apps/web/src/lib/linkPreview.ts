import { withoutSpoilers } from "./markdown";
import type { TextMessage } from "./textChannels";

// Link previews: the server fetches the first link of a message and sends a
// card (image served by the server). null: the page has none.
export type LinkPreview = { url: string; title: string; description?: string; site?: string; image?: string };

// The same rule as the server's (sidecar firstLink).
// Links in spoilers do not count: the card would show them.
const LINK = /https?:\/\/[^\s<>"|]+[^\s<>".,:;!?)\]'*_~|]/;

export const firstLink = (text: string): string | null => LINK.exec(withoutSpoilers(text))?.[0] ?? null;

// A message's preview is asked once per link (an edit can change it).
export const previewKey = (m: TextMessage) => `${m.id} ${firstLink(m.text)}`;

// Messages with a link whose preview is neither known nor asked for.
export const previewsToAsk = (
  msgs: readonly TextMessage[],
  known: Readonly<Record<number, LinkPreview | null>>,
  asked: ReadonlySet<string>,
): TextMessage[] => msgs.filter((m) => !(m.id in known) && firstLink(m.text) && !asked.has(previewKey(m)));

export const previewsRequest = (messages: number[]) => JSON.stringify({ type: "previews", messages });
