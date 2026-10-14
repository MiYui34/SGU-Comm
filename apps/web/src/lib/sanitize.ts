import DOMPurify from "dompurify";

// Mumble messages are HTML from other users. On top of the default
// (XSS-safe) config: links open in a new tab, so clicking one does not drop
// the connection, and only inline images are allowed, so nobody can track
// readers with remote images. Mumble sends images as data: URLs anyway.
DOMPurify.addHook("afterSanitizeAttributes", (node) => {
  if (node.tagName === "A") {
    node.setAttribute("target", "_blank");
    node.setAttribute("rel", "noopener noreferrer");
  }
  if (node.tagName === "IMG" && !node.getAttribute("src")?.startsWith("data:image/")) {
    node.removeAttribute("src");
  }
});

export const sanitize = (html: string) => DOMPurify.sanitize(html);
