// Web Push (see server/internal/push): while the app is closed, the server
// pushes mentions and direct messages to this browser, and a service
// worker (sw.ts, at /sw.js) shows them. It is registered only while push
// is on. A subscription belongs to one account on one server (owner).
import { isDesktop } from "./desktop";
import { safeGet, safeSet } from "./storage";

// "install": iOS has push only for apps on the home screen (16.4+).
export type PushSupport = "ok" | "install" | "none";

export const pushSupport = (e: {
  desktop: boolean;
  dev: boolean; // Vite serves no /sw.js
  serviceWorker: boolean;
  pushManager: boolean;
  notification: boolean;
  ios: boolean;
  standalone: boolean;
}): PushSupport => {
  if (e.desktop || e.dev) return "none";
  if (e.ios && !e.standalone) return "install";
  return e.serviceWorker && e.pushManager && e.notification ? "ok" : "none";
};

export const browserPushSupport = (): PushSupport =>
  pushSupport({
    desktop: isDesktop(),
    dev: import.meta.env.DEV,
    serviceWorker: "serviceWorker" in navigator,
    pushManager: "PushManager" in window,
    notification: "Notification" in window,
    // iPads say they are Macs, but have touch.
    ios: /iPad|iPhone|iPod/.test(navigator.userAgent) || (/Macintosh/.test(navigator.userAgent) && navigator.maxTouchPoints > 1),
    standalone: matchMedia("(display-mode: standalone)").matches || (navigator as { standalone?: boolean }).standalone === true,
  });

// The server's VAPID key (base64url) as the Push API wants it.
export const applicationServerKey = (key: string): Uint8Array<ArrayBuffer> => {
  const b64 = key.replaceAll("-", "+").replaceAll("_", "/") + "=".repeat((4 - (key.length % 4)) % 4);
  return Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
};

// A subscription made with another key (new server keys) must be made again.
export const keyMatches = (current: ArrayBuffer | null, key: string): boolean => {
  if (!current) return false;
  const [a, b] = [new Uint8Array(current), applicationServerKey(key)];
  return a.length === b.length && a.every((v, i) => v === b[i]);
};

export const pushOwner = (base: string, name: string) => `${base} ${name.toLowerCase()}`;

// quiet: text channels set to "nothing" here (no pushes for them).
export const subscriptionBody = (sub: PushSubscriptionJSON, quiet: number[]) => ({ endpoint: sub.endpoint, keys: sub.keys, quiet });

const OWNER_KEY = "pushOwner";

const send = async (base: string, method: "POST" | "DELETE", body: unknown) => {
  const res = await fetch(`${base}/api/push/subscriptions`, {
    method,
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(((await res.json().catch(() => ({}))) as { error?: string }).error ?? `request failed (${res.status})`);
};

const serverKey = async (base: string): Promise<string> => {
  const res = await fetch(`${base}/api/push`, { credentials: "include" });
  const key = res.ok ? ((await res.json().catch(() => ({}))) as { publicKey?: unknown }).publicKey : null;
  if (typeof key !== "string" || !key) throw new Error("this server has no push notifications");
  return key;
};

const registration = async () => navigator.serviceWorker?.getRegistration("/");

// Subscribes (again, if the server's key changed) and tells the server,
// for the session now.
const subscribe = async (base: string, owner: string, quiet: number[]) => {
  const key = await serverKey(base);
  await navigator.serviceWorker.register("/sw.js", { scope: "/" });
  const reg = await navigator.serviceWorker.ready;
  let sub = await reg.pushManager.getSubscription();
  if (sub && !keyMatches(sub.options.applicationServerKey, key)) {
    await sub.unsubscribe();
    sub = null;
  }
  sub ??= await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: applicationServerKey(key) });
  await send(base, "POST", subscriptionBody(sub.toJSON(), quiet));
  safeSet(localStorage, OWNER_KEY, owner);
};

export const pushIsOn = async (base: string, name: string) =>
  safeGet(localStorage, OWNER_KEY) === pushOwner(base, name) && !!(await (await registration())?.pushManager.getSubscription());

// null: on; else why not.
export const turnOnPush = async (base: string, name: string, quiet: number[]): Promise<string | null> => {
  if ((await Notification.requestPermission()) !== "granted") return "The browser does not allow notifications for this site.";
  try {
    await subscribe(base, pushOwner(base, name), quiet);
    return null;
  } catch (err) {
    return `Push notifications did not turn on: ${err instanceof Error ? err.message : err}`;
  }
};

// Also on logout. The server forgets it with the session anyway.
export const turnOffPush = async (base: string) => {
  safeSet(localStorage, OWNER_KEY, "");
  const reg = await registration().catch(() => undefined);
  const sub = await reg?.pushManager.getSubscription().catch(() => null);
  if (sub) await send(base, "DELETE", { endpoint: sub.endpoint }).catch(() => {});
  await sub?.unsubscribe().catch(() => {});
  await reg?.unregister().catch(() => {});
};

// On start and when channel settings change: push that is on for this
// account follows the session and the quiet channels (and comes back if
// the browser dropped the subscription); another account's goes.
export const syncPush = async (base: string, name: string, quiet: number[]) => {
  const owner = safeGet(localStorage, OWNER_KEY);
  if (owner === pushOwner(base, name) && Notification.permission === "granted")
    await subscribe(base, owner, quiet).catch((err) => console.warn("push", err));
  else if (owner || (await registration().catch(() => undefined))) await turnOffPush(base);
};
