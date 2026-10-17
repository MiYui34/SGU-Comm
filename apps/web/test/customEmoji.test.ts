// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import {
  addCustomEmojiImages,
  customEmojiSet,
  customEntries,
  customName,
  emojiFormError,
  emojiNameFromFile,
  knownEmoji,
  parseCustomEmoji,
  type CustomEmoji,
} from "../src/lib/customEmoji";
import { sanitize } from "../src/lib/sanitize";
import { markdown } from "../src/lib/markdown";
import { addRecent, normalEmoji, searchEmoji } from "../src/lib/emojiPicker";
import { escapeHtml } from "@kallisti/protocol";
import { deleteEmojiRequest, emojiTokenRequest, initialTextState, parseServerEvent, reduceText } from "../src/lib/textChannels";

const list: CustomEmoji[] = [
  { name: "party", url: "/api/emoji/1" },
  { name: "top10", url: "/api/emoji/2" },
];
const set = customEmojiSet(list, "https://chat.example");

describe("custom emoji from the server", () => {
  it("parses the list; images must be on the server under /api/emoji/", () => {
    expect(parseCustomEmoji(list)).toEqual(list);
    expect(parseCustomEmoji([{ name: "x", url: "/api/emoji/1" }])).toBeNull();
    expect(parseCustomEmoji([{ name: "ok", url: "https://evil.example/a.png" }])).toBeNull();
    expect(parseCustomEmoji([{ name: "ok", url: "/files/a" }])).toBeNull();
    expect(parseCustomEmoji("nope")).toBeNull();
  });

  it("comes with ready and as an event of its own", () => {
    const ready = parseServerEvent(JSON.stringify({ type: "ready", channels: [], emoji: list }));
    expect(ready).toMatchObject({ type: "ready", emoji: list });
    let state = reduceText(initialTextState, ready!);
    expect(state.emoji).toEqual(list);
    const ev = parseServerEvent(JSON.stringify({ type: "emoji", emoji: [list[1]] }));
    expect(ev).toEqual({ type: "emoji", emoji: [list[1]] });
    state = reduceText(state, ev!);
    expect(state.emoji).toEqual([list[1]]);
    expect(reduceText(initialTextState, parseServerEvent('{"type":"ready","channels":[]}')!).emoji).toEqual([]);
  });

  it("has requests to add (a token) and delete", () => {
    expect(JSON.parse(emojiTokenRequest("party"))).toEqual({ type: "emoji-token", name: "party" });
    expect(JSON.parse(deleteEmojiRequest("party"))).toEqual({ type: "delete-emoji", name: "party" });
  });

  it("gives absolute image URLs by name", () => {
    expect(set.urls.get("party")).toBe("https://chat.example/api/emoji/1");
    expect(set.list[1]).toEqual({ name: "top10", url: "https://chat.example/api/emoji/2" });
  });
});

describe("customName", () => {
  it("is the name in :name:", () => {
    expect(customName(":top10:")).toBe("top10");
    expect(customName("top10")).toBeNull();
    expect(customName(":Top:")).toBeNull();
    expect(customName("👍")).toBeNull();
  });
  it("knownEmoji: Unicode emoji, and custom ones of this server", () => {
    expect(knownEmoji("👍", set)).toBe(true);
    expect(knownEmoji(":party:", set)).toBe(true);
    expect(knownEmoji(":gone:", set)).toBe(false);
  });
});

describe("custom emoji in messages", () => {
  const html = (text: string) => sanitize(markdown(escapeHtml(text)), false, set.urls);
  const img = (name: string, id: number) =>
    `<img class="emoji custom" draggable="false" title=":${name}:" alt=":${name}:" src="https://chat.example/api/emoji/${id}">`;

  it("become images, known names only", () => {
    expect(html("hi :party: :nope: :top10::party:")).toBe(`hi ${img("party", 1)} :nope: ${img("top10", 2)}${img("party", 1)}`);
  });
  it("stay text in code and links", () => {
    expect(html("`:party:` https://x.org/:party:/x")).toBe(
      '<code>:party:</code> <a href="https://x.org/:party:/x" target="_blank" rel="noopener noreferrer">https://x.org/:party:/x</a>',
    );
  });
  it("keep their name from Markdown emphasis", () => {
    expect(markdown(":_ok_: _a_")).toBe(":_ok_: <em>a</em>");
  });
  it("cannot be forged by message HTML (images are added after sanitizing, into text only)", () => {
    const out = sanitize('<img class="emoji custom" src="https://chat.example/api/emoji/1"><a title=":party:">x</a>', false, set.urls);
    expect(out).not.toContain("src=");
    expect(out).toContain('title=":party:"');
  });
  it("addCustomEmojiImages leaves a DOM without names alone", () => {
    const div = document.createElement("div");
    div.textContent = "plain";
    addCustomEmojiImages(div, set.urls);
    expect(div.innerHTML).toBe("plain");
  });
});

describe("custom emoji in the picker and suggestions", () => {
  it("are entries with :name:, found by their name's parts", () => {
    const entries = customEntries(set.list);
    expect(entries[0]).toMatchObject({ emoji: ":party:", name: "party" });
    const more = customEntries(customEmojiSet([{ name: "party_parrot", url: "/api/emoji/3" }], "").list);
    expect(searchEmoji([...entries, ...more], "parrot").map((e) => e.emoji)).toEqual([":party_parrot:"]);
    expect(searchEmoji([...entries, ...more], "par").map((e) => e.emoji)).toEqual([":party:", ":party_parrot:"]);
  });
  it("are kept as they are (digits are no keycaps here)", () => {
    expect(normalEmoji(":top10:")).toBe(":top10:");
    expect(addRecent([], ":top10:")).toEqual([":top10:"]);
  });
});

describe("adding custom emoji", () => {
  const file = (size: number, type = "image/png") => ({ size, type });
  it("checks name and image before the upload", () => {
    expect(emojiFormError("party", file(1000), [])).toBeNull();
    expect(emojiFormError("a", file(1000), [])).toMatch(/2 to 32/);
    expect(emojiFormError("Party", file(1000), [])).toMatch(/lower case/);
    expect(emojiFormError("party", file(1000), list)).toMatch(/exists/);
    expect(emojiFormError("party", null, [])).toMatch(/image/);
    expect(emojiFormError("party", file(300 * 1024), [])).toMatch(/256 KB/);
    expect(emojiFormError("party", file(1000, "image/svg+xml"), [])).toMatch(/PNG/);
    const full = Array.from({ length: 200 }, (_, i) => ({ name: `e${i}`, url: `/api/emoji/${i}` }));
    expect(emojiFormError("party", file(1000), full)).toMatch(/most/);
  });
  it("suggests a name from the file name", () => {
    expect(emojiNameFromFile("Party Parrot.gif")).toBe("party_parrot");
    expect(emojiNameFromFile("über-cool!!.png")).toBe("ber_cool");
    expect(emojiNameFromFile(`${"x".repeat(40)}.png`)).toBe("x".repeat(32));
  });
});
