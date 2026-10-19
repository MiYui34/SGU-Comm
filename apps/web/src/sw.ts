// The service worker, for push notifications only (see lib/push.ts): no
// fetch handler, so it caches nothing and the app loads as without it.
// Built to /sw.js (see vite.config.ts); it must stay one file.
import { clickTarget, noticeOf, windowFor } from "./lib/pushNotice";

// The few worker types used here (the WebWorker lib clashes with DOM).
type Waitable = Event & { waitUntil(p: Promise<unknown>): void };
type AppWindow = { url: string; focused: boolean; focus(): Promise<unknown>; postMessage(m: unknown): void };
type Worker = {
  registration: ServiceWorkerRegistration;
  clients: { matchAll(o: { type: "window"; includeUncontrolled: boolean }): Promise<AppWindow[]>; openWindow(url: string): Promise<unknown> };
  skipWaiting(): Promise<void>;
  addEventListener(type: string, f: (e: Waitable) => void): void;
};
const sw = self as unknown as Worker;
const origin = location.origin;

// A new version takes over at once (it holds no state).
sw.addEventListener("install", () => void sw.skipWaiting());

sw.addEventListener("push", (e) => {
  let data: unknown = null;
  try {
    data = (e as unknown as { data: { json(): unknown } | null }).data?.json();
  } catch {
    // Not JSON: the general text.
  }
  const n = noticeOf(data, origin);
  e.waitUntil(sw.registration.showNotification(n.title, n.options));
});

// A click opens the message: in an open window of the app (which jumps
// there), else in a new one (the server answers message links with the
// app).
sw.addEventListener("notificationclick", (e) => {
  const notification = (e as unknown as { notification: Notification }).notification;
  notification.close();
  const url = clickTarget((notification.data as { url?: unknown } | null)?.url, origin);
  e.waitUntil(
    (async () => {
      const win = windowFor(await sw.clients.matchAll({ type: "window", includeUncontrolled: true }), origin);
      if (!win) return sw.clients.openWindow(url);
      await win.focus();
      win.postMessage({ type: "open", url });
    })(),
  );
});
