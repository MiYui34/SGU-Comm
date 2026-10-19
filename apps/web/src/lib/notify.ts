import { isDesktop } from "./desktop";
import { htmlToText } from "./htmlText";

// Desktop notifications for mentions and private messages: only when the
// user turned them on and the browser allows it, not while deafened or in
// do not disturb, and not for the chat that is open in a visible tab.
export const shouldNotify = (s: {
  enabled: boolean;
  permission: Permission;
  visible: boolean;
  deaf: boolean;
  open: boolean; // the message's chat is the open one
  dnd?: boolean;
}) => s.enabled && s.permission === "granted" && !s.deaf && !s.dnd && !(s.visible && s.open);

const BODY_LENGTH = 140;

// Mumble messages are HTML; a picture alone has no text.
export const notificationBody = (html: string) => htmlToText(html).trim().slice(0, BODY_LENGTH) || "(image)";

// Where notifications come from: the desktop app's own (system
// notifications, see tauri-plugin-notification; WKWebView on macOS has no
// web notifications, WebKitGTK asks the app), the browser's, or none
// (Safari tabs on iOS).
export type NotifierKind = "native" | "web" | "none";

export const notifierKind = (e: { desktop: boolean; web: boolean }): NotifierKind =>
  e.desktop ? "native" : e.web ? "web" : "none";

export type Permission = NotificationPermission | "unsupported";

export type Notifier = {
  permission: () => Permission;
  request: () => Promise<Permission>;
  // A click brings the app forward and runs onClick.
  show: (title: string, body: string, onClick: () => void) => void;
};

// The plugin's permission states ("prompt": not asked yet).
export const nativePermission = (state: string): Permission =>
  state === "granted" || state === "denied" ? state : "default";

export const permissionNote = (permission: Permission, desktop: boolean) =>
  permission === "unsupported"
    ? "This browser has no desktop notifications."
    : permission !== "denied"
      ? null
      : desktop
        ? "The system blocks notifications for Kallisti (see its notification settings)."
        : "The browser blocks notifications for this site.";

// Whether the user sees the page. The desktop app's window stays visible
// behind other windows: there it also needs the focus.
export const chatSeen = (s: { visible: boolean; focused: boolean; desktop: boolean }) =>
  s.visible && (s.focused || !s.desktop);

// Android Chrome refuses the constructor (only service workers may): no
// notification there.
export const webNotifier: Notifier = {
  permission: () => Notification.permission,
  request: () => Notification.requestPermission(),
  show: (title, body, onClick) => {
    try {
      const n = new Notification(title, { body, tag: title, renotify: true } as NotificationOptions);
      n.onclick = () => {
        window.focus();
        onClick();
        n.close();
      };
    } catch {
      // Not possible here.
    }
  },
};

const noNotifier: Notifier = { permission: () => "unsupported", request: async () => "unsupported", show: () => {} };

type Action = { actionId: string; notification?: { id?: unknown } | null };

// The plugin reports a click on a notification as "tap", with its id.
export const tappedId = (e: Action) =>
  e.actionId === "tap" && typeof e.notification?.id === "number" ? e.notification.id : null;

type NativeApi = {
  isPermissionGranted: () => Promise<boolean>;
  requestPermission: () => Promise<string>;
  sendNotification: (options: { id: number; title: string; body: string }) => void;
  onAction: (cb: (e: Action) => void) => Promise<unknown>;
};

const KEEP_CLICKS = 50;

// The app (Rust) brings its window forward on a click; this runs the
// notification's onClick. Clicks are only reported for notifications sent
// after the listener is in place.
export const nativeNotifier = (
  load: () => Promise<NativeApi> = () => import("@tauri-apps/plugin-notification"),
): Notifier => {
  let permission: Permission = "default";
  let nextId = 1;
  const clicks = new Map<number, () => void>();
  const api = load().then(async (n) => {
    permission = (await n.isPermissionGranted()) ? "granted" : "default";
    await n.onAction((e) => {
      const id = tappedId(e);
      const onClick = id === null ? undefined : clicks.get(id);
      if (id !== null && onClick) {
        clicks.delete(id);
        onClick();
      }
    });
    return n;
  });
  api.catch(() => {});
  return {
    permission: () => permission,
    request: () =>
      api.then(async (n) => (permission = nativePermission(await n.requestPermission()))).catch(() => permission),
    show: (title, body, onClick) => {
      const id = nextId++;
      clicks.set(id, onClick);
      if (clicks.size > KEEP_CLICKS) clicks.delete(clicks.keys().next().value as number);
      void api.then((n) => n.sendNotification({ id, title, body })).catch(() => {});
    },
  };
};

let native: Notifier | undefined;

export const notifier = (): Notifier => {
  const kind = notifierKind({ desktop: isDesktop(), web: typeof Notification !== "undefined" });
  return kind === "native" ? (native ??= nativeNotifier()) : kind === "web" ? webNotifier : noNotifier;
};
