import { getContext, setContext } from "svelte";
import type { EmojiEntry } from "./emojiPicker";

// Custom server emoji: a name and an image on the server. In messages and
// as reactions they are ":name:"; Mumble clients see that text.
// url: relative to the server as it comes, absolute in a CustomEmojiSet.
export type CustomEmoji = { name: string; url: string };
export type CustomEmojiSet = { list: CustomEmoji[]; urls: ReadonlyMap<string, string> };

// As the server checks them (server/internal/emoji ValidName), and its
// limits (store.MaxEmoji, files.MaxEmojiBytes, the inline image types).
export const CUSTOM_NAME = "[a-z0-9_]{2,32}";
const NAME = new RegExp(`^${CUSTOM_NAME}$`);
const IN_COLONS = new RegExp(`^:(${CUSTOM_NAME}):$`);
const MAX_EMOJI = 200;
const MAX_BYTES = 256 * 1024;
export const EMOJI_TYPES = ["image/png", "image/gif", "image/webp", "image/jpeg"];

// The images must be on the server: anything else could track readers.
const isEmoji = (v: unknown): v is CustomEmoji =>
  !!v &&
  typeof v === "object" &&
  NAME.test((v as CustomEmoji).name) &&
  typeof (v as CustomEmoji).url === "string" &&
  /^\/api\/emoji\/\d+$/.test((v as CustomEmoji).url);

// A broken entry is left out (null: no list at all).
export const parseCustomEmoji = (v: unknown): CustomEmoji[] | null => (Array.isArray(v) ? v.filter(isEmoji) : null);

export const customEmojiSet = (list: readonly CustomEmoji[], base: string): CustomEmojiSet => {
  const abs = list.map((e) => ({ name: e.name, url: base + e.url }));
  return { list: abs, urls: new Map(abs.map((e) => [e.name, e.url])) };
};

export const customName = (emoji: string): string | null => {
  const m = IN_COLONS.exec(emoji);
  return m ? m[1]! : null;
};

// Unicode emoji (the server checks them), or a custom one of this server
// (recent emoji may be from another server, or deleted).
export const knownEmoji = (emoji: string, set: CustomEmojiSet) => {
  const name = customName(emoji);
  return name === null || set.urls.has(name);
};

const customImage = (name: string, url: string) => {
  const img = document.createElement("img");
  img.className = "emoji custom";
  img.draggable = false;
  img.alt = img.title = `:${name}:`;
  img.src = url;
  return img;
};

// Replaces known ":name:" in a (sanitized) DOM's text with images; not in
// code or links. Attributes are never touched.
export const addCustomEmojiImages = (root: Node, urls: ReadonlyMap<string, string>) => {
  if (urls.size === 0) return;
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const texts: Text[] = [];
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    if (!n.parentElement?.closest("code, pre, a")) texts.push(n as Text);
  }
  for (const node of texts) {
    const parts = node.data.split(new RegExp(`:(${CUSTOM_NAME}):`));
    if (parts.length === 1 || !parts.some((p, i) => i % 2 === 1 && urls.has(p))) continue;
    // Odd parts are names: unknown ones stay text.
    node.replaceWith(...parts.map((p, i) => (i % 2 === 0 ? p : urls.has(p) ? customImage(p, urls.get(p)!) : `:${p}:`)));
  }
};

// For the picker and the ":" suggestions (searched by name).
export const customEntries = (list: readonly CustomEmoji[]): EmojiEntry[] =>
  list.map((e) => ({ emoji: `:${e.name}:`, name: e.name, keywords: "", group: -1 }));

// Why a new emoji cannot be added (null: it can); the server checks again.
export const emojiFormError = (name: string, file: { size: number; type: string } | null, list: readonly CustomEmoji[]): string | null => {
  if (name.length < 2 || name.length > 32) return "A name has 2 to 32 characters.";
  if (!NAME.test(name)) return "A name has only lower case letters, digits and _.";
  if (list.some((e) => e.name === name)) return "An emoji with this name exists.";
  if (list.length >= MAX_EMOJI) return `This server has the most emoji it can have (${MAX_EMOJI}).`;
  if (!file) return "Choose an image.";
  if (!EMOJI_TYPES.includes(file.type)) return "The image must be PNG, GIF, WebP or JPEG.";
  if (file.size > MAX_BYTES) return "The image can be at most 256 KB.";
  return null;
};

// A name from a file name: "Party Parrot.gif" -> "party_parrot".
export const emojiNameFromFile = (file: string) =>
  file
    .replace(/\.[^.]*$/, "")
    .toLowerCase()
    .replace(/[^a-z0-9_]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 32);

// Components below the client get the current server's emoji (as avatars
// do, see avatar.ts).
const KEY = Symbol("customEmoji");
const NONE: CustomEmojiSet = { list: [], urls: new Map() };
export const setCustomEmoji = (get: () => CustomEmojiSet) => setContext(KEY, get);
export const customEmojiFor = (): (() => CustomEmojiSet) => getContext<(() => CustomEmojiSet) | undefined>(KEY) ?? (() => NONE);
