import { describe, expect, it } from "vitest";
import { htmlToLines, htmlToText } from "../src/lib/htmlText";

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

describe("htmlToLines", () => {
  it("keeps line breaks from tags and from the text", () => {
    expect(htmlToLines("one<br>two &amp; <b>three</b>")).toBe("one\ntwo & three");
    expect(htmlToLines("plain\ntext")).toBe("plain\ntext"); // as the Kallisti server sends it
  });

  it("reads a Mumble client's rich text", () => {
    const qt =
      '<!DOCTYPE HTML><html><head><meta name="qrichtext" content="1" /><style type="text/css">p { white-space: pre-wrap; }</style></head>' +
      '<body style="font-size:9pt;">\n<p style="margin:0px;">Hi,   I am <i>Bob</i>.</p>\n<p style="margin:0px;">Second line</p></body></html>';
    expect(htmlToLines(qt)).toBe("Hi, I am Bob.\n\nSecond line");
  });

  it("drops scripts, keeps a lone <, decodes numeric entities", () => {
    expect(htmlToLines("x<script>alert(1)</script>y")).toBe("xy");
    expect(htmlToLines("a < b &#39;&#x41;&lt;i&gt;")).toBe("a < b 'A<i>");
  });

  it("trims lines and keeps at most one empty line", () => {
    expect(htmlToLines("  a  \n\n\n\n b <br><br><br>c")).toBe("a\n\nb\n\nc");
  });
});
