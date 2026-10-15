// Images open in the lightbox on a plain left click; middle and modified
// clicks (new tab, new window, download) stay with the browser.
export const opensLightbox = (e: Pick<MouseEvent, "button" | "ctrlKey" | "metaKey" | "shiftKey" | "altKey">) =>
  e.button === 0 && !e.ctrlKey && !e.metaKey && !e.shiftKey && !e.altKey;
