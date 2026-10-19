// The service worker's side of push notifications (see sw.ts): the server
// sends a short text and a link to the message (see server/internal/push).
// Only the service worker imports this (it must stay one file).

const ICON = "/icons/icon-192.png";

// A link opens on this server only.
export const clickTarget = (url: unknown, origin: string): string => {
  try {
    const u = new URL(typeof url === "string" ? url : "/", origin);
    if (u.origin === origin) return u.href;
  } catch {
    // Not a URL.
  }
  return `${origin}/`;
};

export const noticeOf = (data: unknown, origin: string) => {
  const d = data && typeof data === "object" ? (data as Record<string, unknown>) : {};
  const text = (v: unknown) => (typeof v === "string" && v ? v : undefined);
  return {
    title: "Kallisti",
    options: {
      body: text(d.body) ?? "New activity",
      tag: text(d.tag),
      icon: ICON,
      badge: ICON,
      data: { url: clickTarget(d.url, origin) },
    },
  };
};

// The app's window to use for a click: the focused one, else any (null:
// open a new one).
export const windowFor = <W extends { url: string; focused: boolean }>(windows: readonly W[], origin: string): W | null => {
  const ours = windows.filter((w) => w.url.startsWith(`${origin}/`));
  return ours.find((w) => w.focused) ?? ours[0] ?? null;
};
