import { describe, expect, it } from "vitest";
import { applicationServerKey, keyMatches, pushOwner, pushSupport, subscriptionBody } from "../src/lib/push";
import { clickTarget, noticeOf, windowFor } from "../src/lib/pushNotice";

const env = { desktop: false, dev: false, serviceWorker: true, pushManager: true, notification: true, ios: false, standalone: false };

describe("pushSupport", () => {
  it("is there in browsers with service workers and the Push API", () => {
    expect(pushSupport(env)).toBe("ok");
    expect(pushSupport({ ...env, ios: true, standalone: true })).toBe("ok"); // iOS 16.4+, on the home screen
  });

  it("asks iOS users to add the app to the home screen first", () => {
    expect(pushSupport({ ...env, ios: true, pushManager: false })).toBe("install");
    expect(pushSupport({ ...env, ios: true })).toBe("install");
  });

  it("is not there in the desktop app, in development, or without the APIs", () => {
    expect(pushSupport({ ...env, desktop: true })).toBe("none");
    expect(pushSupport({ ...env, dev: true })).toBe("none");
    expect(pushSupport({ ...env, pushManager: false })).toBe("none");
    expect(pushSupport({ ...env, serviceWorker: false })).toBe("none");
    expect(pushSupport({ ...env, notification: false })).toBe("none");
    expect(pushSupport({ ...env, ios: true, standalone: true, pushManager: false })).toBe("none"); // older iOS
  });
});

describe("keys", () => {
  it("decodes the server's base64url key", () => {
    expect([...applicationServerKey("AQL_-w")]).toEqual([1, 2, 255, 251]);
  });

  it("compares a subscription's key with the server's", () => {
    const key = applicationServerKey("AQL_-w");
    expect(keyMatches(key.buffer, "AQL_-w")).toBe(true);
    expect(keyMatches(key.buffer, "AQL_-A")).toBe(false);
    expect(keyMatches(null, "AQL_-w")).toBe(false);
  });
});

describe("subscriptions", () => {
  it("are sent with the quiet channels and blocked users", () => {
    const json = { endpoint: "https://push.example/x", keys: { p256dh: "P", auth: "A" }, expirationTime: null };
    expect(subscriptionBody(json, { quiet: [3, 4], blocked: ["bob"] })).toEqual({
      endpoint: "https://push.example/x",
      keys: { p256dh: "P", auth: "A" },
      quiet: [3, 4],
      blocked: ["bob"],
    });
  });

  it("belong to one account on one server", () => {
    expect(pushOwner("https://a", "Alice")).toBe(pushOwner("https://a", "alice"));
    expect(pushOwner("https://a", "alice")).not.toBe(pushOwner("https://b", "alice"));
    expect(pushOwner("https://a", "alice")).not.toBe(pushOwner("https://a", "bob"));
  });
});

const ORIGIN = "https://chat.example";

describe("noticeOf", () => {
  it("shows the server's short text and opens its link", () => {
    expect(noticeOf({ body: "Mention in #general from bob", url: "/channels/1/7", tag: "chat-1" }, ORIGIN)).toEqual({
      title: "Kallisti",
      options: { body: "Mention in #general from bob", tag: "chat-1", icon: "/icons/icon-192.png", badge: "/icons/icon-192.png", data: { url: `${ORIGIN}/channels/1/7` } },
    });
  });

  it("has a text for broken data", () => {
    const n = noticeOf("nope", ORIGIN);
    expect(n.options.body).toBe("New activity");
    expect(n.options.data.url).toBe(`${ORIGIN}/`);
  });
});

describe("clickTarget", () => {
  it("stays on this server", () => {
    expect(clickTarget("/channels/1/7", ORIGIN)).toBe(`${ORIGIN}/channels/1/7`);
    expect(clickTarget("https://evil.example/x", ORIGIN)).toBe(`${ORIGIN}/`);
    expect(clickTarget(42, ORIGIN)).toBe(`${ORIGIN}/`);
  });
});

describe("windowFor", () => {
  it("uses an open window of the app, the focused one first", () => {
    const a = { url: `${ORIGIN}/`, focused: false };
    const b = { url: `${ORIGIN}/channels/1/2`, focused: true };
    const other = { url: "https://other.example/", focused: true };
    expect(windowFor([a, b, other], ORIGIN)).toBe(b);
    expect(windowFor([other, a], ORIGIN)).toBe(a);
    expect(windowFor([other], ORIGIN)).toBeNull();
  });
});
