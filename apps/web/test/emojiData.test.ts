import { describe, expect, it } from "vitest";
// @ts-expect-error: a build script without types
import { compact } from "../scripts/emojidata.mjs";

const tones = (base: string, label: string) =>
  [1, 2, 3, 4, 5].map((tone) => ({ emoji: base + String.fromCodePoint(0x1f3fa + tone), label: `${label}: tone ${tone}`, tone }));

describe("compact emoji data", () => {
  const data = [
    { emoji: "👍", label: "thumbs up", tags: ["+1", "thumb"], group: 1, order: 2, version: 0.6, skins: tones("👍", "thumbs up") },
    { emoji: "😀", label: "grinning face", tags: ["face"], group: 0, order: 1, version: 1 },
    { emoji: "🏻", label: "light skin tone", group: 2, order: 3, version: 1 },
    { emoji: "🫝", label: "pickle", group: 4, order: 4, version: 18 },
    { emoji: "🍔", label: "hamburger", group: 4, order: 5, version: 1 },
    { emoji: "🇦", label: "regional indicator A", order: 0, version: 0.6 },
  ];
  const shortcodes = { "1F44D": ["+1", "thumbsup"], "1F600": "grinning" };
  const hex = (e: string) => [...e].map((c) => c.codePointAt(0)!.toString(16).toUpperCase()).join("-");
  const out = compact(
    data.map((e) => ({ ...e, hexcode: hex(e.emoji) })),
    shortcodes,
    17,
  );

  it("keeps emoji up to the Twemoji version, in order, without components", () => {
    expect(out.emoji.map((e: unknown[]) => e[0])).toEqual(["😀", "👍", "🍔"]);
  });

  it("merges smileys and people into one group", () => {
    expect(out.groups.map((g: { name: string }) => g.name)[0]).toBe("Smileys & People");
    const group = (e: string) => out.emoji.find((x: unknown[]) => x[0] === e)[3];
    expect(group("😀")).toBe(group("👍"));
    expect(out.groups[group("🍔")].name).toBe("Food & Drink");
  });

  it("keeps names, keywords with shortcodes, and the five skin tones", () => {
    expect(out.emoji[1]).toEqual(["👍", "thumbs up", "+1 thumb thumbsup", 0, tones("👍", "").map((t) => t.emoji)]);
    expect(out.emoji[0]).toEqual(["😀", "grinning face", "face grinning", 0]);
  });
});
