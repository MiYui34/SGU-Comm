import { describe, expect, it } from "vitest";
import { notificationBody, notificationPermission, shouldNotify, showNotification } from "../src/lib/notify";

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

describe("showNotification", () => {
  it("does not throw where the constructor is not allowed (Android)", () => {
    const original = globalThis.Notification;
    globalThis.Notification = class {
      constructor() {
        throw new TypeError("Illegal constructor");
      }
    } as unknown as typeof Notification;
    expect(() => showNotification("t", "b", () => {})).not.toThrow();
    globalThis.Notification = original;
  });

  it("gives the permission, or denied without the API", () => {
    const original = globalThis.Notification;
    // @ts-expect-error: a browser without notifications
    delete globalThis.Notification;
    expect(notificationPermission()).toBe("denied");
    globalThis.Notification = original;
  });
});
