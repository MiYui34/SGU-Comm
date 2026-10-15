import type { TextMessage } from "./textChannels";

// Link previews: the server fetches the first link of a message and sends a
// card (image served by the server). null: the page has none.
export type LinkPreview = { url: string; title: string; description?: string; site?: string; image?: string };

// The same rule as the server's (sidecar firstLink).
const LINK = /https?:\/\/[^\s<>"]+[^\s<>".,:;!?)\]'*_~]/;

export const firstLink = (text: string): string | null => LINK.exec(text)?.[0] ?? null;

// Messages with a link whose preview is neither known nor asked for.
export const previewsToAsk = (
  msgs: readonly TextMessage[],
  known: Readonly<Record<number, LinkPreview | null>>,
  asked: ReadonlySet<number>,
): number[] => msgs.filter((m) => !(m.id in known) && !asked.has(m.id) && firstLink(m.text)).map((m) => m.id);

export const previewsRequest = (messages: number[]) => JSON.stringify({ type: "previews", messages });
