import { describe, expect, it } from "vitest";
import { channelLink, messageLink, parseMessageLink } from "../src/lib/messageLink";

describe("message links", () => {
  it("make a link to a message on the server", () => {
    expect(messageLink("https://chat.example", 4, 1234)).toBe("https://chat.example/channels/4/1234");
  });

  it("read a link back, only for this server", () => {
    expect(parseMessageLink("https://chat.example/channels/4/1234", "https://chat.example")).toEqual({ chat: 4, message: 1234 });
    expect(parseMessageLink("/channels/4/1234", "https://chat.example")).toEqual({ chat: 4, message: 1234 });
    expect(parseMessageLink("https://other.example/channels/4/1234", "https://chat.example")).toBeNull();
    expect(parseMessageLink("https://chat.example/channels/x/1", "https://chat.example")).toBeNull();
    expect(parseMessageLink("https://chat.example/", "https://chat.example")).toBeNull();
  });

  it("make and read a link to a chat", () => {
    expect(channelLink("https://chat.example", 4)).toBe("https://chat.example/channels/4");
    expect(parseMessageLink("https://chat.example/channels/4", "https://chat.example")).toEqual({ chat: 4 });
    expect(parseMessageLink("https://chat.example/channels/4/", "https://chat.example")).toBeNull();
  });
});
