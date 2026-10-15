import { describe, expect, it } from "vitest";
import { shouldSendTyping, stoppedTyping, typed, typingNames, typingText, TYPING_SEND_MS, TYPING_SHOW_MS } from "../src/lib/typing";
import { parseServerEvent } from "../src/lib/textChannels";

describe("typing", () => {
  it("shows who types in a channel until it runs out, not us", () => {
    let t = typed({}, 1, "bob", 1000);
    t = typed(t, 1, "alice", 2000);
    t = typed(t, 1, "me", 2000);
    t = typed(t, 2, "carol", 2000);
    expect(typingNames(t, 1, 2500, "me")).toEqual(["alice", "bob"]);
    expect(typingNames(t, 1, 1000 + TYPING_SHOW_MS, "me")).toEqual(["alice"]);
    // The message arrives: no longer typing.
    expect(typingNames(stoppedTyping(t, 1, "alice"), 1, 2500, "me")).toEqual(["bob"]);
  });

  it("says it in words", () => {
    expect(typingText([])).toBe("");
    expect(typingText(["bob"])).toBe("bob is typing…");
    expect(typingText(["alice", "bob"])).toBe("alice and bob are typing…");
    expect(typingText(["a", "b", "c"])).toBe("a, b and c are typing…");
    expect(typingText(["a", "b", "c", "d"])).toBe("Several people are typing…");
  });

  it("sends at most every few seconds", () => {
    expect(shouldSendTyping(undefined, 5000)).toBe(true);
    expect(shouldSendTyping(5000, 5000 + TYPING_SEND_MS - 1)).toBe(false);
    expect(shouldSendTyping(5000, 5000 + TYPING_SEND_MS)).toBe(true);
  });

  it("parses the server's event", () => {
    expect(parseServerEvent('{"type":"typing","channel":3,"name":"bob"}')).toEqual({ type: "typing", channel: 3, name: "bob" });
    expect(parseServerEvent('{"type":"typing","channel":3}')).toBeNull();
  });
});
