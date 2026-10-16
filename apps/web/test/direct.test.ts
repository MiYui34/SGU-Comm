import { describe, expect, it } from "vitest";
import { mentionCounts } from "../src/lib/mentions";
import { directChatWith, directRequest, initialTextState, parseServerEvent, reduceText, type TextMessage } from "../src/lib/textChannels";

const ev = (v: object) => parseServerEvent(JSON.stringify(v))!;
const msg = (id: number, channel: number, author: string, text = "hi"): TextMessage => ({ id, channel, author, text, at: id });

describe("direct chats", () => {
  it("come with ready and with dm events", () => {
    let s = reduceText(initialTextState, ev({ type: "ready", channels: [], directs: [{ channel: 7, peer: { id: 20, name: "bob" } }] }));
    expect(s.directs).toEqual({ 7: { id: 20, name: "bob" } });
    s = reduceText(s, ev({ type: "dm", channel: 9, peer: { id: 30, name: "carol" } }));
    expect(directChatWith(s.directs, 30)).toBe(9);
    expect(directChatWith(s.directs, 99)).toBeNull();
  });
  it("are asked for by user id", () => {
    expect(JSON.parse(directRequest(20))).toEqual({ type: "dm", user: 20 });
  });
  it("count every unread message from the other user as a mention", () => {
    let s = reduceText(initialTextState, ev({ type: "ready", channels: [], directs: [{ channel: 7, peer: { id: 20, name: "bob" } }] }));
    s = reduceText(s, ev({ type: "message", message: msg(1, 7, "bob") }));
    s = reduceText(s, ev({ type: "message", message: msg(2, 7, "alice") }));
    s = reduceText(s, ev({ type: "message", message: msg(3, 7, "bob", "no mention here") }));
    expect(mentionCounts(s, {}, "alice")).toEqual({ 7: 2 });
    expect(mentionCounts(s, { 7: 1 }, "alice")).toEqual({ 7: 1 });
  });
});
