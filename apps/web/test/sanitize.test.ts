// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { sanitize } from "../src/lib/sanitize";

const img = (alt: string, code: string) => `<img class="emoji" draggable="false" alt="${alt}" src="/emoji/${code}.png">`;

describe("sanitize with emoji", () => {
  it("makes emoji in text Twemoji images, after sanitizing", () => {
    expect(sanitize("hi 😀 <b>❤️</b>", true)).toBe(`hi ${img("😀", "1f600")} <b>${img("❤️", "2764")}</b>`);
  });
  it("never touches attributes, and attribute text cannot become markup", () => {
    const div = document.createElement("div");
    div.innerHTML = sanitize('<a title="😀 <img src=x onerror=alert(1)>" href="https://x.org/">hi</a>', true);
    expect(div.querySelectorAll("img").length).toBe(0); // the title stays text
    expect(div.querySelector("a")?.title).toBe("😀 <img src=x onerror=alert(1)>");
  });
  it("makes images in code too (blank there without an emoji font)", () => {
    expect(sanitize("<code>😀</code>", true)).toBe(`<code>${img("😀", "1f600")}</code>`);
  });
  it("does not let foreign images in, also with the emoji class", () => {
    expect(sanitize('<img class="emoji" src="/emoji/../api/x.png">', true)).not.toContain("src=");
  });
  it("leaves emoji alone without the option", () => {
    expect(sanitize("hi 😀")).toBe("hi 😀");
  });
});
