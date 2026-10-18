import { describe, expect, it } from "vitest";
import { formatList, minutesError, parseList, parseMessageRules, rulesFrom } from "../src/lib/messageRules";
import { initialTextState, manageRequest, parseServerEvent, reduceText } from "../src/lib/textChannels";

const rules = { newMinutes: 10, words: ["spam", "buy now"], domains: ["junk.example"] };

describe("parseMessageRules", () => {
  it("takes the server's rules", () => {
    expect(parseMessageRules(rules)).toEqual(rules);
    expect(parseMessageRules({ ...rules, words: null })).toBeNull();
    expect(parseMessageRules({ ...rules, words: [1] })).toBeNull();
    expect(parseMessageRules({ ...rules, newMinutes: "10" })).toBeNull();
    expect(parseMessageRules(null)).toBeNull();
  });
});

describe("lists", () => {
  it("are one entry per line, without empty lines", () => {
    expect(parseList(" spam \n\n  buy now\r\n")).toEqual(["spam", "buy now"]);
    expect(parseList("")).toEqual([]);
    expect(formatList(["spam", "buy now"])).toBe("spam\nbuy now");
  });
});

describe("minutesError", () => {
  it("allows 0 (off) to a week of whole minutes", () => {
    for (const ok of ["0", "10", " 10080 "]) expect(minutesError(ok)).toBeNull();
    for (const bad of ["10081", "-1", "1.5", "abc", ""]) expect(minutesError(bad)).not.toBeNull();
  });
});

describe("rulesFrom", () => {
  it("makes the rules of the form's fields", () => {
    expect(rulesFrom(" 10 ", "spam\nbuy now", "junk.example\n")).toEqual(rules);
  });
});

describe("message rules events and requests", () => {
  it("keeps the rules an admin asked for", () => {
    const s = reduceText(initialTextState, parseServerEvent(JSON.stringify({ type: "message-rules", rules }))!);
    expect(s.messageRules).toEqual(rules);
    expect(parseServerEvent('{"type":"message-rules"}')).toBeNull();
    expect(JSON.parse(manageRequest({ messageRules: true }))).toEqual({ type: "message-rules" });
    expect(JSON.parse(manageRequest({ setMessageRules: rules }))).toEqual({ type: "set-message-rules", rules });
  });
});
