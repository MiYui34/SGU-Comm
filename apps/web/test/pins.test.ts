import { describe, expect, it } from "vitest";
import { initialTextState, parseServerEvent, pinRequest, pinsRequest, reduceText, type TextMessage } from "../src/lib/textChannels";

const msg = (id: number, pinned?: number): TextMessage => ({ id, channel: 1, author: "a", text: `m${id}`, at: id, ...(pinned ? { pinned } : {}) });
const ev = (v: object) => parseServerEvent(JSON.stringify(v))!;

describe("pinned messages", () => {
  it("are asked for and set", () => {
    expect(JSON.parse(pinRequest(5, true))).toEqual({ type: "pin", message: 5, on: true });
    expect(JSON.parse(pinsRequest(1))).toEqual({ type: "pins", channel: 1 });
  });

  it("come as a list per channel", () => {
    const s = reduceText(initialTextState, ev({ type: "pins", channel: 1, messages: [msg(2, 20), msg(1, 10)] }));
    expect(s.pins[1]?.map((m) => m.id)).toEqual([2, 1]);
  });

  it("change with message-pinned: the loaded message and the list (newest pin first)", () => {
    let s = reduceText(initialTextState, ev({ type: "history", channel: 1, messages: [msg(1), msg(2)], more: false }));
    s = reduceText(s, ev({ type: "pins", channel: 1, messages: [msg(1, 10)] }));
    s = reduceText(s, ev({ type: "message-pinned", message: msg(2, 20) }));
    expect(s.messages[1]?.find((m) => m.id === 2)?.pinned).toBe(20);
    expect(s.pins[1]?.map((m) => m.id)).toEqual([2, 1]);
    s = reduceText(s, ev({ type: "message-pinned", message: msg(1) }));
    expect(s.pins[1]?.map((m) => m.id)).toEqual([2]);
    expect(s.messages[1]?.find((m) => m.id === 1)?.pinned).toBeUndefined();
  });

  it("leave a list that was not loaded alone", () => {
    const s = reduceText(initialTextState, ev({ type: "message-pinned", message: msg(3, 30) }));
    expect(s.pins[1]).toBeUndefined();
  });
});

describe("pinned messages follow edits, deletes and resyncs", () => {
  const start = () => reduceText(initialTextState, ev({ type: "pins", channel: 1, messages: [msg(1, 10)] }));
  it("drops a deleted one", () => {
    expect(reduceText(start(), ev({ type: "message-deleted", channel: 1, id: 1 })).pins[1]).toEqual([]);
  });
  it("shows an edit", () => {
    const s = reduceText(start(), ev({ type: "message-edited", message: { ...msg(1, 10), text: "new" } }));
    expect(s.pins[1]?.[0]?.text).toBe("new");
  });
  it("forgets them on a resync (they load again)", () => {
    expect(reduceText(start(), ev({ type: "resync" })).pins).toEqual({});
  });
});
