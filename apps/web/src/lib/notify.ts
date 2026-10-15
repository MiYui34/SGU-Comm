import { htmlToText } from "./htmlText";

// Desktop notifications for mentions and private messages: only when the
// user turned them on and the browser allows it, not while deafened, and
// not for the chat that is open in a visible tab.
export const shouldNotify = (s: {
  enabled: boolean;
  permission: NotificationPermission;
  visible: boolean;
  deaf: boolean;
  open: boolean; // the message's chat is the open one
}) => s.enabled && s.permission === "granted" && !s.deaf && !(s.visible && s.open);

const BODY_LENGTH = 140;

// Mumble messages are HTML; a picture alone has no text.
export const notificationBody = (html: string) => htmlToText(html).trim().slice(0, BODY_LENGTH) || "(image)";

// "denied" where the browser has no notifications (Safari tabs on iOS).
export const notificationPermission = (): NotificationPermission =>
  typeof Notification === "undefined" ? "denied" : Notification.permission;

// Shows a notification; a click brings the app forward and runs onClick.
// Android Chrome refuses the constructor (only service workers may): no
// notification there.
export const showNotification = (title: string, body: string, onClick: () => void) => {
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
};
