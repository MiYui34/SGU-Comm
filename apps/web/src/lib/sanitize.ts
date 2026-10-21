import DOMPurify from "dompurify";
import { addEmojiImages } from "./emoji";
import { addCustomEmojiImages, type EmojiImages } from "./customEmoji";

// Mumble messages are HTML from other users. On top of the default
// (XSS-safe) config: links open in a new tab, so clicking one does not drop
// the connection, and only inline images are allowed, so nobody can track
// readers with remote images. Mumble sends images as data: URLs anyway.
// A link to a text channel (from markdown: href "#" and a numeric
// data-channel) stays in the tab; Mumble HTML could carry the attribute too.
export const isChannelLink = (a: Element) => a.getAttribute("href") === "#" && /^\d+$/.test(a.getAttribute("data-channel") ?? "");

DOMPurify.addHook("afterSanitizeAttributes", (node) => {
  if (node.tagName === "A" && !isChannelLink(node)) {
    node.setAttribute("target", "_blank");
    node.setAttribute("rel", "noopener noreferrer");
  }
  if (node.tagName === "IMG" && !node.getAttribute("src")?.startsWith("data:image/")) {
    node.removeAttribute("src");
  }
});

// emoji: emoji become Twemoji images, added after sanitizing (into text
// only), so nothing from the message can make or change them. The same for
// custom emoji (":name:" and "<:name:id>" -> image URL).
export const sanitize = (html: string, emoji = false, custom?: EmojiImages) => {
  if (!emoji && !custom) return DOMPurify.sanitize(html);
  const fragment = DOMPurify.sanitize(html, { RETURN_DOM_FRAGMENT: true });
  if (custom) addCustomEmojiImages(fragment, custom);
  if (emoji) addEmojiImages(fragment);
  const div = document.createElement("div");
  div.append(fragment);
  return div.innerHTML;
};
