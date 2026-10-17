import { describe, expect, it } from "vitest";
import { parseSearch } from "../src/lib/search";

const chats = new Map([
  ["general", 1],
  ["lounge 2", 7],
]);

describe("parseSearch", () => {
  it("takes from: and in: out of the words, as in Discord", () => {
    expect(parseSearch("pizza from:bob in:#general tonight", chats)).toEqual({ text: "pizza tonight", author: "bob", channel: 1 });
    expect(parseSearch("FROM:@Bob", chats)).toEqual({ text: "", author: "Bob" });
    expect(parseSearch('in:"Lounge 2" from:"two words"', chats)).toEqual({ text: "", author: "two words", channel: 7 });
    expect(parseSearch("in:@General", chats)).toEqual({ text: "", channel: 1 }); // direct chats by name
    expect(parseSearch('in:@"Lounge 2" hi', chats)).toEqual({ text: "hi", channel: 7 });
  });

  it("finds nothing in a chat that does not exist", () => {
    expect(parseSearch("in:#nowhere hi", chats)).toEqual({ text: "hi", channel: -1 });
  });

  it("leaves other words with a colon alone", () => {
    expect(parseSearch("time: 10:30 https://x.org from:", chats)).toEqual({ text: "time: 10:30 https://x.org from:" });
  });
});
