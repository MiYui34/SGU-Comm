import { describe, expect, it } from "vitest";
import { htmlToText } from "../src/lib/htmlText";

describe("htmlToText", () => {
  it("drops tags and decodes entities", () => {
    expect(htmlToText("<b>Gaming</b> &amp; chill<br>no &lt;spam&gt;")).toBe("Gaming & chill no <spam>");
  });

  it("collapses whitespace and trims", () => {
    expect(htmlToText("  <p>a</p>\n\n<p>b</p>  ")).toBe("a b");
  });

  it("drops script and style content", () => {
    expect(htmlToText("x<script>alert(1)</script><style>p{}</style>y")).toBe("xy");
  });
});
