import { describe, expect, it } from "vitest";
import {
  deleteMessageRequest,
  editMessageRequest,
  historyRequest,
  initialTextState,
  markReadRequest,
  linkify,
  manageRequest,
  parseServerEvent,
  reduceText,
  searchRequest,
  sendRequest,
  toChatMessage,
  unreadTextIds,
  uploadTokenRequest,
  voiceChatRequest,
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

  it("shows errors about channel changes as a notice, not in the chat", () => {
    const failed = reduceText(ready, { type: "error", message: "name in use", about: "manage" });
    expect(failed.lastError).toBeNull();
    expect(failed.notice).toEqual({ text: "name in use", n: 1 });
    expect(reduceText(failed, { type: "error", message: "x", about: "manage" }).notice?.n).toBe(2);
  });

  it("reads the topic of an error", () => {
    expect(parseServerEvent('{"type":"error","message":"m","about":"manage"}')).toEqual({ type: "error", message: "m", about: "manage" });
    expect(parseServerEvent('{"type":"error","message":"m","about":"other"}')).toEqual({ type: "error", message: "m" });
  });
});

describe("requests", () => {
  it("builds history and send requests", () => {
    expect(JSON.parse(historyRequest(1))).toEqual({ type: "history", channel: 1 });
    expect(JSON.parse(historyRequest(1, 40))).toEqual({ type: "history", channel: 1, before: 40 });
    expect(JSON.parse(sendRequest(2, "hi"))).toEqual({ type: "send", channel: 2, text: "hi" });
    expect(JSON.parse(sendRequest(2, "yes", undefined, 7))).toEqual({ type: "send", channel: 2, text: "yes", reply: 7 });
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

describe("robustness", () => {
  const ready = reduceText(initialTextState, { type: "ready", channels: [{ id: 1, name: "general" }] });

  it("parses resync and verified authors", () => {
    expect(parseServerEvent('{"type":"resync"}')).toEqual({ type: "resync" });
    const ev = parseServerEvent(JSON.stringify({ type: "message", message: { ...msg(1), verified: true } }));
    expect(ev?.type === "message" && ev.message.verified).toBe(true);
  });

  it("forgets loaded history on resync and counts the resyncs", () => {
    let s = reduceText(ready, { type: "history", channel: 1, messages: [msg(1)], more: false });
    s = reduceText(s, { type: "resync" });
    expect(s.messages).toEqual({});
    expect(s.loaded).toEqual({});
    expect(s.resyncs).toBe(1);
    expect(s.channels).toEqual(ready.channels);
  });

  it("marks verified authors on chat messages", () => {
    expect(toChatMessage({ ...msg(77), verified: true }).verified).toBe(true);
    expect(toChatMessage(msg(78)).verified).toBe(false);
  });
});

describe("voice channel chats", () => {
  it("knows whether the server stores voice channel chats", () => {
    const on = reduceText(initialTextState, parseServerEvent('{"type":"ready","channels":[],"voiceChats":true}')!);
    const off = reduceText(initialTextState, parseServerEvent('{"type":"ready","channels":[]}')!);
    expect(on.voiceChats).toBe(true);
    expect(off.voiceChats).toBe(false);
  });

  it("maps a voice channel to the channel that holds its chat", () => {
    const ev = parseServerEvent('{"type":"voice-chat","voice":5,"channel":42}');
    expect(ev).toEqual({ type: "voice-chat", voice: 5, channel: 42 });
    expect(reduceText(initialTextState, ev!).voiceChannels[5]).toBe(42);
    expect(parseServerEvent('{"type":"voice-chat","voice":"5","channel":42}')).toBeNull();
  });

  it("asks for a voice channel's chat", () => {
    expect(JSON.parse(voiceChatRequest(5))).toEqual({ type: "voice-chat", voice: 5 });
  });
});

describe("managing text channels", () => {
  it("takes a new channel list", () => {
    const ev = parseServerEvent('{"type":"channels","channels":[{"id":2,"name":"games"}]}');
    expect(ev).toEqual({ type: "channels", channels: [{ id: 2, name: "games" }] });
    expect(reduceText(initialTextState, ev!).channels).toEqual([{ id: 2, name: "games" }]);
  });

  it("makes the requests", () => {
    expect(JSON.parse(manageRequest({ create: "games" }))).toEqual({ type: "create-channel", name: "games" });
    expect(JSON.parse(manageRequest({ rename: 2, name: "x" }))).toEqual({ type: "rename-channel", channel: 2, name: "x" });
    expect(JSON.parse(manageRequest({ remove: 2 }))).toEqual({ type: "delete-channel", channel: 2 });
    expect(JSON.parse(manageRequest({ order: [3, 2] }))).toEqual({ type: "order-channels", channels: [3, 2] });
    expect(JSON.parse(manageRequest({ moderator: 5, on: true }))).toEqual({ type: "set-moderator", user: 5, moderator: true });
    expect(JSON.parse(manageRequest({ topic: 2, text: "hi" }))).toEqual({ type: "set-topic", channel: 2, text: "hi" });
  });

  it("reads topics", () => {
    const ev = parseServerEvent('{"type":"channels","channels":[{"id":2,"name":"games","topic":"Play"}]}');
    expect(ev).toEqual({ type: "channels", channels: [{ id: 2, name: "games", topic: "Play" }] });
  });
});

describe("roles", () => {
  it("come with ready and change later", () => {
    const ready = parseServerEvent('{"type":"ready","channels":[],"voiceChats":true,"roles":{"admins":[3],"moderators":[]}}');
    const s = reduceText(initialTextState, ready!);
    expect(s.roles).toEqual({ admins: [3], moderators: [] });
    const changed = parseServerEvent('{"type":"roles","admins":[3],"moderators":[5]}');
    expect(reduceText(s, changed!).roles).toEqual({ admins: [3], moderators: [5] });
    expect(parseServerEvent('{"type":"roles","admins":"x","moderators":[]}')).toBeNull();
  });

});

describe("edited and deleted messages", () => {
  const ready = reduceText(initialTextState, { type: "ready", channels: [{ id: 1, name: "general" }] });
  it("replaces and removes them", () => {
    let s = reduceText(ready, { type: "message", message: msg(1) });
    s = reduceText(s, { type: "message", message: msg(2) });
    const edited = parseServerEvent(JSON.stringify({ type: "message-edited", message: { ...msg(1), text: "new", edited: 5 } }));
    s = reduceText(s, edited!);
    expect(s.messages[1]?.map((m) => [m.id, m.text, m.edited])).toEqual([
      [1, "new", 5],
      [2, msg(2).text, undefined],
    ]);
    const deleted = parseServerEvent('{"type":"message-deleted","channel":1,"id":2}');
    expect(reduceText(s, deleted!).messages[1]?.map((m) => m.id)).toEqual([1]);
  });

  it("builds the requests", () => {
    expect(JSON.parse(editMessageRequest(4, "x"))).toEqual({ type: "edit-message", message: 4, text: "x" });
    expect(JSON.parse(deleteMessageRequest(4))).toEqual({ type: "delete-message", message: 4 });
  });

  it("keeps the source for editing in the chat line", () => {
    const line = toChatMessage({ ...msg(7), text: "a <b>", edited: 9 });
    expect(line.source).toEqual({ id: 7, text: "a <b>" });
    expect(line.edited).toBe(9);
  });

  it("reads replies and keeps their quote", () => {
    const raw = { ...msg(8), reply: { id: 7, author: "bob", text: "q" } };
    expect(parseServerEvent(JSON.stringify({ type: "message", message: raw }))).toEqual({ type: "message", message: raw });
    expect(toChatMessage(raw).reply).toEqual({ id: 7, author: "bob", text: "q" });
  });
});

describe("read state from the server", () => {
  const raw = JSON.stringify({
    type: "ready",
    channels: [{ id: 1, name: "general" }, { id: 2, name: "random" }],
    voiceChats: true,
    reads: { "1": 4 },
    summary: [
      { channel: 1, latest: 9, mentions: 2 },
      { channel: 2, latest: 3, mentions: 0 },
      { channel: 7, voice: 5, latest: 6, mentions: 1 },
    ],
  });

  it("keeps read marks and the summary, and learns voice chats", () => {
    const s = reduceText(initialTextState, parseServerEvent(raw)!);
    expect(s.reads).toEqual({ 1: 4 });
    expect(s.summary).toEqual({ 1: { latest: 9, mentions: 2 }, 2: { latest: 3, mentions: 0 }, 7: { latest: 6, mentions: 1 } });
    expect(s.voiceChannels[5]).toBe(7);
  });

  it("marks channels unread by the summary, without loaded messages", () => {
    const s = reduceText(initialTextState, parseServerEvent(raw)!);
    expect([...unreadTextIds(s, { 1: 4, 2: 3 })]).toEqual([1]);
  });

  it("builds the mark-read request", () => {
    expect(JSON.parse(markReadRequest(1, 9))).toEqual({ type: "mark-read", channel: 1, message: 9 });
  });
});

describe("search", () => {
  it("keeps the latest results", () => {
    const ev = parseServerEvent(JSON.stringify({ type: "search-results", text: "pizza", messages: [msg(3)] }));
    expect(ev).toEqual({ type: "search-results", text: "pizza", messages: [msg(3)] });
    const s = reduceText(initialTextState, ev!);
    expect(s.search).toEqual({ text: "pizza", messages: [msg(3)] });
    expect(JSON.parse(searchRequest({ text: "pizza" }))).toEqual({ type: "search", text: "pizza" });
    expect(JSON.parse(searchRequest({ text: "", author: "bob", channel: 3 }))).toEqual({ type: "search", text: "", author: "bob", channel: 3 });
  });
});

describe("search results follow edits and deletes", () => {
  it("patches and drops them", () => {
    let s = reduceText(initialTextState, { type: "search-results", text: "x", messages: [msg(1), msg(2)] });
    s = reduceText(s, { type: "message-edited", message: { ...msg(1), text: "new" } });
    s = reduceText(s, { type: "message-deleted", channel: 1, id: 2 });
    expect(s.search?.messages.map((m) => [m.id, m.text])).toEqual([[1, "new"]]);
  });
});
