// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { escapeHtml } from "@kallisti/protocol";
import { markdown, toggleSpoiler } from "../src/lib/markdown";

describe("toggleSpoiler", () => {
  const html = (text: string) => {
    const div = document.createElement("div");
    div.innerHTML = markdown(escapeHtml(text));
    return div;
  };

  it("shows a hidden spoiler, and hides it again", () => {
    const div = html("||secret **x**||");
    const bold = div.querySelector("strong")!;
    const span = div.querySelector(".spoiler")!;
    expect(toggleSpoiler(bold)).toBe(true);
    expect(span.classList.contains("shown")).toBe(true);
    expect(span.hasAttribute("aria-label")).toBe(false); // the text is read out
    expect(span.getAttribute("aria-expanded")).toBe("true");
    expect(span.getAttribute("tabindex")).toBe("0"); // still a button
    expect(toggleSpoiler(bold)).toBe(true);
    expect(span.classList.contains("shown")).toBe(false);
    expect(span.getAttribute("aria-label")).toBe("Spoiler");
    expect(span.getAttribute("aria-expanded")).toBe("false");
  });

  it("lets a link in a shown spoiler open", () => {
    const div = html("||see https://example.com||");
    const link = div.querySelector("a")!;
    expect(toggleSpoiler(link)).toBe(true); // hidden: the click shows it
    expect(toggleSpoiler(link)).toBe(false); // shown: the link opens
    expect(div.querySelector(".spoiler")!.classList.contains("shown")).toBe(true);
  });

  it("does nothing outside spoilers", () => {
    expect(toggleSpoiler(html("plain").firstChild!.parentElement!)).toBe(false);
  });
});
