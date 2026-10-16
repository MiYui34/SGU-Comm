// A new version of the web app on the server: its index.html names another
// main script (Vite names it by its content, so any change shows there).

const MODULE_SCRIPT = /<script\b[^>]*\btype="module"[^>]*\bsrc="([^"]+)"/;

export const mainScript = (html: string): string | null => MODULE_SCRIPT.exec(html)?.[1] ?? null;

// current: the running page's main script (null: unknown). Built pages
// only: in development the script is the source.
export const isNewVersion = (current: string | null, html: string): boolean => {
  const latest = mainScript(html);
  return !!current && !!latest && current.startsWith("/assets/") && latest !== current;
};

// The running page's main script.
export const runningScript = (): string | null =>
  document.querySelector<HTMLScriptElement>('script[type="module"][src]')?.getAttribute("src") ?? null;

// Asks the server (base: its address) whether there is a new version.
export const checkForUpdate = async (base: string): Promise<boolean> => {
  try {
    const res = await fetch(`${base}/`, { cache: "no-store" });
    return res.ok && isNewVersion(runningScript(), await res.text());
  } catch {
    return false; // offline: next time
  }
};
