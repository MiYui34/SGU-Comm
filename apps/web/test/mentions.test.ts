import { describe, expect, it } from "vitest";
import { highlightMentions, mentionCounts, mentions, mentionsMe } from "../src/lib/mentions";
import { initialTextState, type TextMessage, type TextState } from "../src/lib/textChannels";

describe("mentions", () => {
  it("finds @name as a whole name, any case", () => {
    expect(mentions("@lain hey how are you", "lain")).toBe(true);
    expect(mentions("hi @Lain!", "lain")).toBe(true);
    expect(mentions("@lainey hi", "lain")).toBe(false);
    expect(mentions("mail@lain.example", "lain")).toBe(false);
    expect(mentions("lain hi", "lain")).toBe(false);
    // Escaped text: entities end in ";", which is not a word break.
    expect(mentions("a&amp;@lain", "lain")).toBe(false);
  });

  it("never counts our own messages", () => {
    expect(mentionsMe("@lain hi", "bob", "lain")).toBe(true);
    expect(mentionsMe("@lain hi", "Lain", "lain")).toBe(false);
    expect(mentionsMe("@lain hi", null, "lain")).toBe(true); // server texts
  });

  it("works on escaped HTML and names with spaces or symbols", () => {
    expect(mentions("@lain soy, hi", "lain soy")).toBe(true);
    expect(mentions("@a&amp;b hi", "a&b")).toBe(true);
  });

  it("highlights @name outside tags only", () => {
    expect(highlightMentions("hi @Lain!", "lain")).toBe('hi <span class="mention">@Lain</span>!');
    const link = '<a href="https://x.example/@lain">https://x.example/@lain</a>';
    expect(highlightMentions(link, "lain")).toBe(link);
  });
});

describe("mentionCounts", () => {
  const msg = (id: number, channel: number, author: string, text: string): TextMessage => ({
    id, channel, author, text, at: 0,
  });
  const state: TextState = {
    ...initialTextState,
    messages: {
      1: [msg(1, 1, "bob", "@lain old"), msg(2, 1, "bob", "@lain new"), msg(3, 1, "lain", "@lain me myself")],
      2: [msg(4, 2, "bob", "no mention"), msg(5, 2, "carol", "@LAIN hi")],
    },
  };

  it("counts unseen mentions of me per channel, not my own", () => {
    expect(mentionCounts(state, { 1: 1 }, "lain")).toEqual({ 1: 1, 2: 1 });
    expect(mentionCounts(state, { 1: 3, 2: 5 }, "lain")).toEqual({});
  });
});
