import { describe, expect, it } from "vitest";
import {
  historyRequest,
  initialTextState,
  linkify,
  parseServerEvent,
  reduceText,
  sendRequest,
  toChatMessage,
  unreadTextIds,
  uploadTokenRequest,
  httpBase,
  formatSize,
  type TextMessage,
} from "../src/lib/textChannels";

const msg = (id: number, channel = 1, author = "alice"): TextMessage => ({
  id,
  channel,
  author,
  text: `m${id}`,
  at: id * 1000,
});

describe("parseServerEvent", () => {
  it("accepts known events", () => {
    expect(parseServerEvent('{"type":"ready","channels":[{"id":1,"name":"general"}]}')).toEqual({
      type: "ready",
      channels: [{ id: 1, name: "general" }],
    });
  });

  it("rejects broken or unknown input", () => {
    expect(parseServerEvent("nope")).toBeNull();
    expect(parseServerEvent('{"type":"dance"}')).toBeNull();
    expect(parseServerEvent('{"type":"ready","channels":"x"}')).toBeNull();
    expect(parseServerEvent('{"type":"message","message":{"id":"1"}}')).toBeNull();
  });
});

describe("reduceText", () => {
  const ready = reduceText(initialTextState, {
    type: "ready",
    channels: [{ id: 1, name: "general" }],
  });

  it("is unavailable until ready", () => {
    expect(initialTextState.available).toBe(false);
    expect(ready.available).toBe(true);
    expect(ready.channels).toEqual([{ id: 1, name: "general" }]);
  });

  it("merges history pages and live messages in id order without duplicates", () => {
    let s = reduceText(ready, { type: "message", message: msg(5) });
    s = reduceText(s, { type: "history", channel: 1, messages: [msg(3), msg(4), msg(5)], more: true });
    s = reduceText(s, { type: "history", channel: 1, messages: [msg(1), msg(2)], more: false });
    s = reduceText(s, { type: "message", message: msg(6) });
    expect(s.messages[1]?.map((m) => m.id)).toEqual([1, 2, 3, 4, 5, 6]);
    expect(s.loaded[1]).toBe(true);
    expect(s.more[1]).toBe(false);
  });

  it("keeps channels apart", () => {
    const s = reduceText(ready, { type: "message", message: msg(1, 2) });
    expect(s.messages[1]).toBeUndefined();
    expect(s.messages[2]).toHaveLength(1);
  });

  it("stores the last error", () => {
    expect(reduceText(ready, { type: "error", message: "nope" }).lastError).toBe("nope");
  });
});

describe("requests", () => {
  it("builds history and send requests", () => {
    expect(JSON.parse(historyRequest(1))).toEqual({ type: "history", channel: 1 });
    expect(JSON.parse(historyRequest(1, 40))).toEqual({ type: "history", channel: 1, before: 40 });
    expect(JSON.parse(sendRequest(2, "hi"))).toEqual({ type: "send", channel: 2, text: "hi" });
  });
});

describe("linkify", () => {
  it("turns URLs in escaped text into links", () => {
    expect(linkify("see https://example.org/a?b=1&amp;c=2 now")).toBe(
      'see <a href="https://example.org/a?b=1&amp;c=2">https://example.org/a?b=1&amp;c=2</a> now',
    );
  });

  it("leaves a trailing dot or bracket outside the link", () => {
    expect(linkify("(https://example.org).")).toBe(
      '(<a href="https://example.org">https://example.org</a>).',
    );
  });

  it("ignores other schemes", () => {
    expect(linkify("javascript:alert(1)")).toBe("javascript:alert(1)");
  });
});

describe("toChatMessage", () => {
  it("escapes the plain text and keeps the author", () => {
    const m = toChatMessage({ ...msg(1), text: "<b>hi</b> https://x.org" });
    expect(m).toMatchObject({
      actor: null,
      actorName: "alice",
      text: '&lt;b&gt;hi&lt;/b&gt; <a href="https://x.org">https://x.org</a>',
      at: 1000,
    });
  });
});

describe("toChatMessage cache", () => {
  it("returns the same object for the same message", () => {
    const m = msg(1);
    expect(toChatMessage(m)).toBe(toChatMessage(m));
  });
});

describe("unreadTextIds", () => {
  it("lists channels with messages newer than the last seen one", () => {
    let s = reduceText(initialTextState, {
      type: "ready",
      channels: [
        { id: 1, name: "a" },
        { id: 2, name: "b" },
        { id: 3, name: "c" },
      ],
    });
    s = reduceText(s, { type: "message", message: msg(5, 1) });
    s = reduceText(s, { type: "message", message: msg(6, 2) });
    expect(unreadTextIds(s, { 1: 5 })).toEqual(new Set([2]));
  });
});

describe("uploads", () => {
  it("parses messages with an attachment", () => {
    const ev = parseServerEvent(
      JSON.stringify({
        type: "message",
        message: { ...msg(1), upload: { url: "/files/k/cat.png", name: "cat.png", type: "image/png", size: 3 } },
      }),
    );
    expect(ev?.type === "message" && ev.message.upload?.name).toBe("cat.png");
  });

  it("drops a broken attachment", () => {
    expect(parseServerEvent(JSON.stringify({ type: "message", message: { ...msg(1), upload: { url: 1 } } }))).toBeNull();
  });

  it("parses the upload token event", () => {
    expect(parseServerEvent('{"type":"upload-token","token":"abc"}')).toEqual({ type: "upload-token", token: "abc" });
  });

  it("sends an upload key with a message", () => {
    expect(JSON.parse(sendRequest(1, "", "key1"))).toEqual({ type: "send", channel: 1, text: "", upload: "key1" });
    expect(JSON.parse(uploadTokenRequest())).toEqual({ type: "upload-token" });
  });

  it("puts the attachment with an absolute URL on the chat message", () => {
    const m = toChatMessage(
      { ...msg(9), upload: { url: "/files/k/cat.png", name: "cat.png", type: "image/png", size: 3 } },
      "http://srv:8080",
    );
    expect(m.upload).toEqual({ url: "http://srv:8080/files/k/cat.png", name: "cat.png", type: "image/png", size: 3 });
  });
});

describe("httpBase", () => {
  it("turns the WebSocket URL into the server's HTTP origin", () => {
    expect(httpBase("ws://localhost:18081")).toBe("http://localhost:18081");
    expect(httpBase("wss://chat.example/ws")).toBe("https://chat.example");
  });
});

describe("formatSize", () => {
  it("writes readable sizes", () => {
    expect(formatSize(512)).toBe("512 B");
    expect(formatSize(1536)).toBe("1.5 KB");
    expect(formatSize(1258291)).toBe("1.2 MB");
  });
});

describe("attachment URL check", () => {
  const withUrl = (url: string) => toChatMessage({ ...msg(Math.random() * 1e9), upload: { url, name: "x", type: "image/png", size: 1 } }, "https://srv.example");

  it("keeps files on our server", () => {
    expect(withUrl("/files/k/x.png").upload?.url).toBe("https://srv.example/files/k/x.png");
  });

  it("drops URLs that leave the server or the files path", () => {
    expect(withUrl("@evil.example/x").upload).toBeUndefined();
    expect(withUrl(".evil.example/x").upload).toBeUndefined();
    expect(withUrl("//evil.example/files/x").upload).toBeUndefined();
    expect(withUrl("javascript:alert(1)").upload).toBeUndefined();
    expect(withUrl("/api/upload").upload).toBeUndefined();
  });
});
