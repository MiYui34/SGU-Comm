import { describe, expect, it } from "vitest";
import { initialTextState, parseServerEvent, reduceText, toChatMessage, type TextMessage } from "../src/lib/textChannels";
import { reactedBy } from "../src/lib/reactions";

const msg = (id: number, extra: Partial<TextMessage> = {}): TextMessage => ({ id, channel: 1, author: "a", text: "x", at: 1, ...extra });

describe("reactions", () => {
  it("come with messages and as events", () => {
    const withReactions = JSON.stringify({ type: "message", message: msg(1, { reactions: [{ emoji: "👍", users: ["bob"] }] }) });
    expect(parseServerEvent(withReactions)).not.toBeNull();
    expect(parseServerEvent(JSON.stringify({ type: "message", message: { ...msg(1), reactions: [{ emoji: 1 }] } }))).toBeNull();
    expect(parseServerEvent('{"type":"reactions","channel":1,"message":2,"reactions":[{"emoji":"🎉","users":["a","b"]}]}')).toEqual({
      type: "reactions",
      channel: 1,
      message: 2,
      reactions: [{ emoji: "🎉", users: ["a", "b"] }],
    });
  });

  it("an event changes the loaded message only", () => {
    let state = reduceText(initialTextState, { type: "history", channel: 1, messages: [msg(1), msg(2)], more: false });
    const before = state.messages[1]![0];
    state = reduceText(state, { type: "reactions", channel: 1, message: 2, reactions: [{ emoji: "👍", users: ["bob"] }] });
    expect(state.messages[1]![1]!.reactions).toEqual([{ emoji: "👍", users: ["bob"] }]);
    expect(state.messages[1]![0]).toBe(before);
    expect(reduceText(state, { type: "reactions", channel: 1, message: 99, reactions: [] })).toBe(state);
    expect(toChatMessage(state.messages[1]![1]!).reactions).toEqual([{ emoji: "👍", users: ["bob"] }]);
  });

  it("knows our own, whatever the case of the name", () => {
    expect(reactedBy({ emoji: "👍", users: ["Bob", "carol"] }, "bob")).toBe(true);
    expect(reactedBy({ emoji: "👍", users: ["carol"] }, "bob")).toBe(false);
  });
});
