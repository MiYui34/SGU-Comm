// A small Markdown subset for chat messages, on text that is already
// escaped: the only tags in the result are the ones made here.

// http(s) URLs become links. Trailing punctuation and a closing bracket stay
// outside, like in Discord.
const URL_PATTERN = /https?:\/\/[^\s<\u0000]+[^\s<.,:;!?)\]'"\u0000]/g;

export const linkify = (escaped: string) => escaped.replace(URL_PATTERN, (url) => `<a href="${url}">${url}</a>`);

const NUL = "\u0000";

// Quote lines ("> text", escaped) in a row become one quote.
const quotes = (s: string) => {
  const out: string[] = [];
  let quote: string[] = [];
  const flush = () => {
    if (quote.length) out.push(`<blockquote>${quote.join("\n")}</blockquote>`);
    quote = [];
  };
  for (const line of s.split("\n")) {
    const m = /^&gt; ?(.*)$/.exec(line);
    if (m) {
      quote.push(m[1]!);
      continue;
    }
    if (quote.length) {
      flush();
      out.push(line); // directly after the quote: no line break
      continue;
    }
    out.push(out.length ? `\n${line}` : line);
  }
  flush();
  return out.join("");
};

const inline = (s: string) =>
  s
    .replace(/\*\*\*(?=\S)([^\n]*?\S)\*\*\*/g, "<strong><em>$1</em></strong>")
    .replace(/\*\*(?=\S)([^\n]*?\S)\*\*/g, "<strong>$1</strong>")
    .replace(/~~(?=\S)([^\n]*?\S)~~/g, "<del>$1</del>")
    .replace(/(^|[^*\w])\*(?=[^\s*])([^\n*]*?[^\s*])\*(?![*\w])/g, "$1<em>$2</em>")
    // Not inside words: snake_case stays.
    .replace(/(^|[^\w])_(?=[^\s_])([^\n_]*?[^\s_])_(?!\w)/g, "$1<em>$2</em>");

export const markdown = (escaped: string): string => {
  // Code and links are set aside first: nothing in them is formatted.
  const kept: string[] = [];
  const keep = (html: string) => `${NUL}${kept.push(html) - 1}${NUL}`;
  let s = escaped.replaceAll(NUL, "");
  s = s.replace(/```(?:[\w+-]*\n)?([\s\S]*?)\n?```/g, (_, code: string) => keep(`<pre><code>${code}</code></pre>`));
  s = s.replace(/`([^`\n]+)`/g, (_, code: string) => keep(`<code>${code}</code>`));
  s = s.replace(URL_PATTERN, (url) => keep(`<a href="${url}">${url}</a>`));
  s = inline(quotes(s));
  return s.replace(/\u0000(\d+)\u0000/g, (_, i: string) => kept[Number(i)]!);
};
