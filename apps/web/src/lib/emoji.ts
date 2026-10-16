import { parse } from "@twemoji/parser";

// Emoji as Twemoji images (72x72 PNG, served with the web app; see
// scripts/twemoji.mjs), so they look the same everywhere, also where emoji
// fonts are blocked (Lockdown Mode). The emoji stays in the alt text, for
// copying and screen readers.
export const EMOJI_BASE = `${import.meta.env.BASE_URL}emoji/`;

export type EmojiPart = { text: string } | { emoji: string; url: string };

// Plain text in parts: text, and emoji with their image.
export const emojiParts = (text: string, base = EMOJI_BASE): EmojiPart[] => {
  const parts: EmojiPart[] = [];
  let at = 0;
  for (const e of parse(text, { buildUrl: (code) => `${base}${code}.png` })) {
    const [from, to] = e.indices;
    const last = parts.at(-1);
    // A variation selector after an emoji that has its own image ("✅" +
    // U+FE0F): the parser gives it alone, without one. It stays with the
    // emoji (in the alt text).
    if (/^\uFE0F+$/.test(e.text)) {
      if (from === at && last && "emoji" in last) last.emoji += e.text;
      else parts.push({ text: text.slice(at, to) });
      at = to;
      continue;
    }
    if (from > at) parts.push({ text: text.slice(at, from) });
    parts.push({ emoji: e.text, url: e.url });
    at = to;
  }
  if (at < text.length) parts.push({ text: text.slice(at) });
  return parts;
};

// Replaces the emoji in a (sanitized) DOM's text with images, also in code
// (without an emoji font, as in Lockdown Mode, text emoji are blank).
// Attributes are never touched.
export const addEmojiImages = (root: Node) => {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const texts: Text[] = [];
  for (let n = walker.nextNode(); n; n = walker.nextNode()) texts.push(n as Text);
  for (const node of texts) {
    const parts = emojiParts(node.data);
    if (!parts.some((p) => "emoji" in p)) continue;
    node.replaceWith(
      ...parts.map((p) => {
        if (!("emoji" in p)) return p.text;
        const img = document.createElement("img");
        img.className = "emoji";
        img.draggable = false;
        img.alt = p.emoji;
        img.src = p.url;
        return img;
      }),
    );
  }
};
