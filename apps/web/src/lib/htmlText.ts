const ENTITIES: Record<string, string> = { amp: "&", lt: "<", gt: ">", quot: '"', apos: "'", nbsp: " " };

const codePoint = (n: number) => (n > 0 && n <= 0x10ffff ? String.fromCodePoint(n) : null);

// Named entities above and numeric ones (&#39; &#x41;); others stay.
const decodeEntities = (text: string) =>
  text.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (match, e: string) =>
    (e[0] === "#" ? codePoint(Number(e.slice(1).replace(/^x/i, "0x"))) : ENTITIES[e]) ?? match,
  );

// Tags out (with the content of script, style and head), line-break tags
// as `br`. Tags start with a letter, "/" or "!": "a < b" is no tag.
const stripTags = (html: string, br: string) =>
  decodeEntities(
    html
      .replace(/<(script|style|head)[^>]*>[\s\S]*?<\/\1>/gi, "")
      .replace(/<br\s*\/?>|<\/p>|<\/div>|<\/li>/gi, br)
      .replace(/<[a-z/!][^>]*>/gi, ""),
  );

// Plain text from Mumble's HTML (channel descriptions), for tooltips and
// the channel topic. The result is shown as text, never as HTML.
export const htmlToText = (html: string) => stripTags(html, " ").replace(/\s+/g, " ").trim();

// Plain text with its lines (user comments). Line breaks in the text stay
// too: the Kallisti server sends comments as text with "\n".
export const htmlToLines = (html: string) =>
  stripTags(html, "\n")
    .split("\n")
    .map((line) => line.replace(/\s+/g, " ").trim())
    .join("\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
