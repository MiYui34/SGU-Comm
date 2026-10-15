import { describe, expect, it } from "vitest";
import { escapeHtml } from "@kallisti/protocol";
import { markdown } from "../src/lib/markdown";

const md = (text: string) => markdown(escapeHtml(text));

describe("markdown", () => {
  it("formats bold, italic and strikethrough", () => {
    expect(md("**bold** and *it* and _it_ and ~~gone~~")).toBe(
      "<strong>bold</strong> and <em>it</em> and <em>it</em> and <del>gone</del>",
    );
    expect(md("***both***")).toBe("<strong><em>both</em></strong>");
  });

  it("leaves things alone that are not formatting", () => {
    expect(md("snake_case_name and 2 * 3 * 4")).toBe("snake_case_name and 2 * 3 * 4");
    expect(md("** not bold **")).toBe("** not bold **");
    expect(md("a**b")).toBe("a**b");
  });

  it("keeps code literal", () => {
    expect(md("run `rm *.tmp *now*` please")).toBe("run <code>rm *.tmp *now*</code> please");
    expect(md("```\nlet x = **y**;\n```")).toBe("<pre><code>let x = **y**;</code></pre>");
    expect(md("```js\nf()\n```")).toBe("<pre><code>f()</code></pre>");
    expect(md("```one line```")).toBe("<pre><code>one line</code></pre>");
  });

  it("makes quotes from lines starting with >", () => {
    expect(md("> said this\nmy answer")).toBe("<blockquote>said this</blockquote>my answer");
    expect(md("> one\n> two")).toBe("<blockquote>one\ntwo</blockquote>");
  });

  it("links URLs without formatting inside them", () => {
    expect(md("see https://x.org/a_b_c *now*")).toBe(
      '<a href="https://x.org/a_b_c">https://x.org/a_b_c</a> <em>now</em>'.replace(/^/, "see "),
    );
    expect(md("`https://x.org`")).toBe("<code>https://x.org</code>");
  });

  it("never lets HTML through", () => {
    expect(md("**<img src=x onerror=alert(1)>**")).toBe("<strong>&lt;img src=x onerror=alert(1)&gt;</strong>");
    expect(md("`<b>`")).toBe("<code>&lt;b&gt;</code>");
    expect(md("\u0000" + "0\u0000")).not.toContain("\u0000");
  });
});
