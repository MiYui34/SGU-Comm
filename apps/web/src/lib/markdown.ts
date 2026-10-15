// A small Markdown subset for chat messages, on text that is already
// escaped: the only tags in the result are the ones made here.

// http(s) URLs become links. Trailing punctuation, a closing bracket and
// Markdown markers stay outside, like in Discord; so does an escaped
// closing bracket or quote ("<https://…>").
const URL_PATTERN = /https?:\/\/[^\s<\u0000]+[^\s<.,:;!?)\]'"*_~\u0000]/g;

const link = (url: string) => {
  const entity = /&(?:gt|quot|#39)$/.exec(url)?.[0] ?? "";
  const href = url.slice(0, url.length - entity.length);
  return { html: `<a href="${href}">${href}</a>`, rest: entity };
};

export const linkify = (escaped: string) =>
  escaped.replace(URL_PATTERN, (url) => {
    const { html, rest } = link(url);
    return html + rest;
  });

const NUL = "\u0000";

const inline = (s: string) =>
  s
    .replace(/\*\*\*(?=\S)([^\n]*?\S)\*\*\*/g, "<strong><em>$1</em></strong>")
    .replace(/\*\*(?=\S)([^\n]*?\S)\*\*/g, "<strong>$1</strong>")
    .replace(/~~(?=\S)([^\n]*?\S)~~/g, "<del>$1</del>")
    .replace(/(^|[^*\w])\*(?=[^\s*])([^\n*]*?[^\s*])\*(?![*\w])/g, "$1<em>$2</em>")
    // Not inside words: snake_case stays.
    .replace(/(^|[^\w])_(?=[^\s_])([^\n_]*?[^\s_])_(?!\w)/g, "$1<em>$2</em>");

// Quote lines ("> text", escaped) in a row become one quote. Formatting
// stays inside a quote or outside it.
const blocks = (s: string) => {
  const parts: { quote: boolean; lines: string[] }[] = [];
  for (const line of s.split("\n")) {
    const m = /^&gt;(?: (.*))?$/.exec(line);
    const quote = m !== null;
    const last = parts.at(-1);
    if (last && last.quote === quote) last.lines.push(quote ? (m[1] ?? "") : line);
    else parts.push({ quote, lines: [quote ? (m[1] ?? "") : line] });
  }
  return parts
    .map((p) => (p.quote ? `<blockquote>${inline(p.lines.join("\n"))}</blockquote>` : inline(p.lines.join("\n"))))
    .join("");
};

export const markdown = (escaped: string): string => {
  // Code and links are set aside first: nothing in them is formatted.
  const kept: string[] = [];
  const keep = (html: string) => `${NUL}${kept.push(html) - 1}${NUL}`;
  let s = escaped.replaceAll(NUL, "");
  s = s.replace(/```(?:[\w+-]*\n)?([\s\S]*?)\n?```/g, (_, code: string) => keep(`<pre><code>${code}</code></pre>`));
  s = s.replace(/`([^`\n\u0000]+)`/g, (_, code: string) => keep(`<code>${code}</code>`));
  s = s.replace(URL_PATTERN, (url) => {
    const { html, rest } = link(url);
    return keep(html) + rest;
  });
  s = blocks(s);
  return s.replace(/\u0000(\d+)\u0000/g, (_, i: string) => kept[Number(i)]!);
};
