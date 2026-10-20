// The desktop app's web view has no tabs: target="_blank" does nothing in
// WKWebView (macOS) and WebKitGTK (Linux) and opens a bare WebView2 window
// on Windows, and a plain link to the server would replace the app. Such
// links open in the system browser instead.

// The URL to open in the system browser for a link, or null: it stays in
// the app (own pages, other schemes).
export const externalUrl = (href: string, appOrigin: string): string | null => {
  let u: URL;
  try {
    u = new URL(href, appOrigin);
  } catch {
    return null;
  }
  return /^https?:$/.test(u.protocol) && u.origin !== appOrigin ? u.href : null;
};

// Clicks (and middle clicks) on such links call open. Runs after the app's
// own handlers: a click they handled (channel and message links, the
// lightbox) is prevented already. Returns a function that stops it.
export const openLinksOutside = (open: (url: string) => unknown, doc: Document = document) => {
  const onClick = (e: MouseEvent) => {
    if (e.defaultPrevented || e.button > 1) return;
    const a = (e.target as Element | null)?.closest?.("a[href]");
    const url = a && externalUrl(a.getAttribute("href")!, doc.location.origin);
    if (!url) return;
    e.preventDefault();
    open(url);
  };
  doc.addEventListener("click", onClick);
  doc.addEventListener("auxclick", onClick);
  return () => {
    doc.removeEventListener("click", onClick);
    doc.removeEventListener("auxclick", onClick);
  };
};
