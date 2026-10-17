// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { escapeHtml } from "@kallisti/protocol";
import { markdown, revealSpoiler } from "../src/lib/markdown";

describe("revealSpoiler", () => {
  const html = (text: string) => {
    const div = document.createElement("div");
    div.innerHTML = markdown(escapeHtml(text));
    return div;
  };

  it("shows a hidden spoiler once, then it is plain text", () => {
    const div = html("||secret **x**||");
    const bold = div.querySelector("strong")!;
    expect(revealSpoiler(bold)).toBe(true);
    const span = div.querySelector(".spoiler")!;
    expect(span.classList.contains("shown")).toBe(true);
    expect(span.hasAttribute("role") || span.hasAttribute("aria-label")).toBe(false);
    expect(span.getAttribute("tabindex")).toBe("-1"); // focus is not lost
    expect(revealSpoiler(bold)).toBe(false); // now a link inside works
  });

  it("does nothing outside spoilers", () => {
    expect(revealSpoiler(html("plain").firstChild!.parentElement!)).toBe(false);
  });
});
