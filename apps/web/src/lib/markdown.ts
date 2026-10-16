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
    .map((p) => {
      const html = inline(p.lines.join("\n")).replaceAll("\n", "<br>");
      return p.quote ? `<blockquote>${html}</blockquote>` : html;
    })
    .join("");
};

// "#name" of a text channel (not inside a word, a URL or an entity such
// as "&#39;"). Names as the server allows them: letters, digits, _ and -.
const CHANNEL_PATTERN = /(^|[^\p{L}\p{N}_&#/-])#([\p{L}\p{N}_-]+)/gu;

// channels: text channel ids by name, for links to them.
export const markdown = (escaped: string, channels?: ReadonlyMap<string, number>): string => {
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
  if (channels?.size) {
    s = s.replace(CHANNEL_PATTERN, (all, before: string, name: string) => {
      const id = channels.get(name.toLowerCase());
      return id === undefined ? all : before + keep(`<a href="#" data-channel="${id}">#${name}</a>`);
    });
  }
  s = blocks(s);
  return s.replace(/\u0000(\d+)\u0000/g, (_, i: string) => kept[Number(i)]!);
};
