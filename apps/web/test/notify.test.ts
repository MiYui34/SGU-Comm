import { describe, expect, it } from "vitest";
import {
  chatSeen,
  nativeNotifier,
  nativePermission,
  notificationBody,
  notifier,
  notifierKind,
  permissionNote,
  shouldNotify,
  tappedId,
  webNotifier,
} from "../src/lib/notify";

const base = { enabled: true, permission: "granted" as NotificationPermission, visible: false, deaf: false, open: false };

describe("shouldNotify", () => {
  it("notifies when on, allowed and not looking at that chat", () => {
    expect(shouldNotify(base)).toBe(true);
    expect(shouldNotify({ ...base, visible: true })).toBe(true); // another chat is open
  });

  it("stays quiet when off, not allowed, deafened, or that chat is open and visible", () => {
    expect(shouldNotify({ ...base, enabled: false })).toBe(false);
    expect(shouldNotify({ ...base, permission: "default" })).toBe(false);
    expect(shouldNotify({ ...base, deaf: true })).toBe(false);
    expect(shouldNotify({ ...base, visible: true, open: true })).toBe(false);
    expect(shouldNotify({ ...base, visible: false, open: true })).toBe(true); // tab hidden
  });
});

describe("notificationBody", () => {
  it("is plain, short text", () => {
    expect(notificationBody("hi &amp; <b>bye</b>")).toBe("hi & bye");
    expect(notificationBody("x".repeat(300))).toHaveLength(140);
    expect(notificationBody('<img src="data:x">')).toBe("(image)");
  });
});

describe("webNotifier", () => {
  it("does not throw where the constructor is not allowed (Android)", () => {
    const original = globalThis.Notification;
    globalThis.Notification = class {
      constructor() {
        throw new TypeError("Illegal constructor");
      }
    } as unknown as typeof Notification;
    expect(() => webNotifier.show("t", "b", () => {})).not.toThrow();
    globalThis.Notification = original;
  });
});

describe("notifier", () => {
  it("is the desktop app's own, else the browser's, else none", () => {
    expect(notifierKind({ desktop: true, web: true })).toBe("native");
    expect(notifierKind({ desktop: true, web: false })).toBe("native");
    expect(notifierKind({ desktop: false, web: true })).toBe("web");
    expect(notifierKind({ desktop: false, web: false })).toBe("none");
  });

  it("is unsupported without the API", () => {
    const original = globalThis.Notification;
    // @ts-expect-error: a browser without notifications
    delete globalThis.Notification;
    expect(notifier().permission()).toBe("unsupported");
    globalThis.Notification = original;
  });
});

describe("permissions", () => {
  it("maps the plugin's states", () => {
    expect(nativePermission("granted")).toBe("granted");
    expect(nativePermission("denied")).toBe("denied");
    expect(nativePermission("prompt")).toBe("default");
    expect(nativePermission("prompt-with-rationale")).toBe("default");
  });

  it("explains what blocks them", () => {
    expect(permissionNote("granted", false)).toBeNull();
    expect(permissionNote("default", true)).toBeNull();
    expect(permissionNote("unsupported", false)).toMatch(/browser has no/);
    expect(permissionNote("denied", false)).toMatch(/browser blocks/);
    expect(permissionNote("denied", true)).toMatch(/system/);
  });
});

describe("chatSeen", () => {
  it("needs a visible page, and in the desktop app also the focus", () => {
    expect(chatSeen({ visible: true, focused: false, desktop: false })).toBe(true);
    expect(chatSeen({ visible: false, focused: true, desktop: false })).toBe(false);
    expect(chatSeen({ visible: true, focused: true, desktop: true })).toBe(true);
    expect(chatSeen({ visible: true, focused: false, desktop: true })).toBe(false);
  });
});

describe("nativeNotifier", () => {
  type Action = Parameters<typeof tappedId>[0];
  const fakeApi = (granted: boolean) => {
    const sent: { id?: number; title: string; body?: string }[] = [];
    let onAction: (e: Action) => void = () => {};
    const api = {
      isPermissionGranted: async () => granted,
      requestPermission: async () => "granted" as const,
      sendNotification: (o: { id?: number; title: string; body?: string }) => void sent.push(o),
      onAction: async (cb: (e: Action) => void) => {
        onAction = cb;
        return { unregister: async () => {} };
      },
    };
    return { api, sent, tap: (e: Action) => onAction(e) };
  };
  const settle = () => new Promise((r) => setTimeout(r));

  it("reads and asks for the permission", async () => {
    const { api } = fakeApi(false);
    const n = nativeNotifier(async () => api);
    await settle();
    expect(n.permission()).toBe("default");
    expect(await n.request()).toBe("granted");
    expect(n.permission()).toBe("granted");
  });

  it("sends title and body, and a click runs that notification's handler once", async () => {
    const { api, sent, tap } = fakeApi(true);
    const n = nativeNotifier(async () => api);
    await settle();
    expect(n.permission()).toBe("granted");
    const clicked: string[] = [];
    n.show("a", "1", () => clicked.push("a"));
    n.show("b", "2", () => clicked.push("b"));
    await settle();
    expect(sent.map(({ title, body }) => [title, body])).toEqual([["a", "1"], ["b", "2"]]);
    tap({ actionId: "tap", notification: { id: sent[1]!.id } });
    tap({ actionId: "tap", notification: { id: sent[1]!.id } });
    tap({ actionId: "other", notification: { id: sent[0]!.id } });
    expect(clicked).toEqual(["b"]);
  });

  it("still sends when clicks cannot be reported", async () => {
    const { api, sent } = fakeApi(true);
    const n = nativeNotifier(async () => ({ ...api, onAction: () => Promise.reject(new Error("not allowed")) }));
    await settle();
    expect(n.permission()).toBe("granted");
    n.show("a", "1", () => {});
    await settle();
    expect(sent).toHaveLength(1);
  });

  it("stays usable when the plugin is missing", async () => {
    const n = nativeNotifier(() => Promise.reject(new Error("no plugin")));
    await settle();
    expect(n.permission()).toBe("default");
    expect(await n.request()).toBe("default");
    expect(() => n.show("t", "b", () => {})).not.toThrow();
  });
});

describe("tappedId", () => {
  it("is the clicked notification's id", () => {
    expect(tappedId({ actionId: "tap", notification: { id: 3 } })).toBe(3);
    expect(tappedId({ actionId: "reply", notification: { id: 3 } })).toBeNull();
    expect(tappedId({ actionId: "tap" })).toBeNull();
  });
});
