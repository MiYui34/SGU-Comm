// Suggestions while typing: "@name", "#channel" and ":emoji" in the message
// box, "from:" and "in:" in the search box. Each finds the word being typed
// before the caret (start: where it begins, query: what is typed after the
// marker); replaceTyped puts the choice in its place.
export type Typed<K extends string> = { start: number; kind: K; query: string };

// After a space or an opening mark, not inside a word, an address or a URL.
// ":emoji" needs two letters, as in Discord (no list for "10:30" or ":)").
const WORD = /(?:^|[\s(\[{,])(?:(@)([^\s@]*)|(#)([\p{L}\p{N}_-]*)|(:)([\w+-]{2,}))$/u;

export const typedWord = (text: string, caret: number): Typed<"@" | "#" | ":"> | null => {
  const m = WORD.exec(text.slice(0, caret));
  if (!m) return null;
  const kind = (m[1] ?? m[3] ?? m[5]) as "@" | "#" | ":";
  const query = m[2] ?? m[4] ?? m[6] ?? "";
  return { start: caret - query.length - 1, kind, query };
};

const FILTER = /(?:^|\s)(from|in):(?:"([^"]*)|(\S*))$/i;

export const typedFilter = (text: string, caret: number): Typed<"from" | "in"> | null => {
  const m = FILTER.exec(text.slice(0, caret));
  if (!m) return null;
  const kind = m[1]!.toLowerCase() as "from" | "in";
  const query = (m[2] ?? m[3] ?? "").replace(/^[#@]/, "");
  return { start: caret - m[0].trimStart().length, kind, query };
};

// The choice and a space after it (a space already there is kept, not
// doubled); the caret goes after the space.
export const replaceTyped = (text: string, start: number, caret: number, insert: string) => {
  const rest = text.slice(caret);
  const spaced = /^\s/u.test(rest) ? insert : `${insert} `;
  return { text: text.slice(0, start) + spaced + rest, caret: start + insert.length + 1 };
};

// A search filter value: names with spaces in quotes.
export const quoted = (name: string) => (/\s/.test(name) ? `"${name}"` : name);

// A key while a list of n is open (active: the chosen one); null: not for
// the list.
export const listKey = (key: string, n: number, active: number): { active: number } | "pick" | "close" | null => {
  if (key === "ArrowDown" || key === "ArrowUp") return { active: (active + (key === "ArrowDown" ? 1 : n - 1)) % n };
  if (key === "Enter" || key === "Tab") return "pick";
  return key === "Escape" ? "close" : null;
};
