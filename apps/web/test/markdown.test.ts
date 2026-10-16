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
    expect(md("> one\n> two")).toBe("<blockquote>one<br>two</blockquote>");
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

  it("ends a link before a closing marker or an escaped bracket", () => {
    const a = '<a href="https://x.org">https://x.org</a>';
    expect(md("**https://x.org**")).toBe(`<strong>${a}</strong>`);
    expect(md("_https://x.org_")).toBe(`<em>${a}</em>`);
    expect(md("<https://x.org>")).toBe(`&lt;${a}&gt;`);
    expect(md('"https://x.org"')).toBe(`&quot;${a}&quot;`);
  });

  it("keeps a code block inside inline code", () => {
    expect(md("`a ```b``` c`")).not.toContain("\u0000");
  });

  it("keeps formatting inside a quote", () => {
    expect(md("> a *b\nc*")).toBe("<blockquote>a *b</blockquote>c*");
  });

  it("needs a space after > for a quote (not >.<)", () => {
    expect(md(">.< oops")).toBe("&gt;.&lt; oops");
  });
});


describe("line breaks and channel links", () => {
  const channels = new Map([["random", 5], ["game-night", 7]]);
  const mdc = (text: string) => markdown(escapeHtml(text), channels);
  it("keeps line breaks, also in quotes; code blocks keep theirs", () => {
    expect(md("one\ntwo")).toBe("one<br>two");
    expect(md("> a\n> b\nc")).toBe("<blockquote>a<br>b</blockquote>c");
    expect(md("```\nx\ny\n```")).toBe("<pre><code>x\ny</code></pre>");
  });
  it("links #names of existing text channels", () => {
    expect(mdc("see #random and #game-night!")).toBe(
      'see <a href="#" data-channel="5">#random</a> and <a href="#" data-channel="7">#game-night</a>!',
    );
    expect(mdc("#nope, a#random, it's")).toBe("#nope, a#random, it&#39;s");
    expect(mdc("https://example.org/#random")).toBe('<a href="https://example.org/#random">https://example.org/#random</a>');
    expect(mdc("`#random`")).toBe("<code>#random</code>");
  });
});

describe("channel links with other letters", () => {
  it("links names with non-ASCII letters and underscores", () => {
    const channels = new Map([["café", 3], ["_misc", 4]]);
    expect(markdown(escapeHtml("#café and #_misc"), channels)).toBe(
      '<a href="#" data-channel="3">#café</a> and <a href="#" data-channel="4">#_misc</a>',
    );
  });
});
