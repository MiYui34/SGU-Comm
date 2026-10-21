import { TEXT_STYLE } from "./emojiText";
import { customName, parseRef } from "./customEmoji";

// The emoji picker's data (public/emoji-data.json, see
// scripts/emojidata.mjs) and the logic around it.
export type EmojiGroup = { name: string; icon: string };
export type EmojiEntry = { emoji: string; name: string; keywords: string; group: number; skins?: string[] };
export type EmojiData = { groups: EmojiGroup[]; emoji: EmojiEntry[] };

const isString = (v: unknown): v is string => typeof v === "string";

export const parseEmojiData = (raw: unknown): EmojiData => {
  const r = raw as { groups?: unknown; emoji?: unknown } | null;
  if (!r || !Array.isArray(r.groups) || !Array.isArray(r.emoji)) return { groups: [], emoji: [] };
  const groups = r.groups.filter((g): g is EmojiGroup => isString(g?.name) && isString(g?.icon));
  const emoji = r.emoji.flatMap((e): EmojiEntry[] => {
    if (!Array.isArray(e)) return [];
    const [emoji, name, keywords, group, skins] = e;
    if (!isString(emoji) || !isString(name) || !isString(keywords) || !Number.isInteger(group) || !groups[group]) return [];
    const entry: EmojiEntry = { emoji, name, keywords, group };
    if (Array.isArray(skins) && skins.length === 5 && skins.every(isString)) entry.skins = skins;
    return [entry];
  });
  return { groups, emoji };
};

let loading: Promise<EmojiData> | null = null;
// Loaded once, when a picker first opens. A failed load is tried again.
export const loadEmojiData = (url = `${import.meta.env.BASE_URL}emoji-data.json`): Promise<EmojiData> =>
  (loading ??= fetch(url)
    .then((res) => (res.ok ? res.json() : Promise.reject(new Error(`HTTP ${res.status}`))))
    .then(parseEmojiData)
    .catch((err) => {
      loading = null;
      throw err;
    }));

const words = (s: string) => s.toLowerCase().split(/[\s_:]+/).filter(Boolean);

// Every word of the search must start a word of the name or keywords.
// Names that start with the search come first.
export const searchEmoji = (list: EmojiEntry[], query: string): EmojiEntry[] => {
  const want = words(query);
  if (want.length === 0) return [];
  const hits = list.filter((e) => {
    const have = words(`${e.name} ${e.keywords}`);
    return want.every((w) => have.some((h) => h.startsWith(w)));
  });
  const first = want.join(" ");
  const starts = (e: EmojiEntry) => e.name.toLowerCase().startsWith(first);
  return [...hits.filter(starts), ...hits.filter((e) => !starts(e))];
};

// tone: 0 none, 1-5 light to dark.
export const withTone = (e: EmojiEntry, tone: number): string => (tone > 0 && e.skins?.[tone - 1]) || e.emoji;

const textStyle = (cp: number) => TEXT_STYLE.some(([from, to]) => cp >= from && cp <= to);
const skinTone = (cp: number | undefined) => cp !== undefined && cp >= 0x1f3fb && cp <= 0x1f3ff;

// The server's form of an emoji (server/internal/emoji Normal): the fully
// qualified one, with the variation selector after symbols that look like
// text without it, except before a skin tone. Custom emoji (":name:",
// "<:name:id>") stay as they are.
export const normalEmoji = (s: string): string => {
  if (customName(s) !== null || parseRef(s)) return s;
  const cps = [...s].map((c) => c.codePointAt(0)!).filter((cp) => cp !== 0xfe0f && cp !== 0xfe0e);
  return cps
    .map((cp, i) => String.fromCodePoint(cp) + (textStyle(cp) && !skinTone(cps[i + 1]) ? "\uFE0F" : ""))
    .join("");
};

const MAX_RECENT = 24;

export const addRecent = (recent: string[], emoji: string): string[] => {
  const e = normalEmoji(emoji);
  return [e, ...recent.filter((r) => r !== e)].slice(0, MAX_RECENT);
};

export const parseRecent = (raw: string | null): string[] => {
  try {
    const v: unknown = JSON.parse(raw ?? "[]");
    return Array.isArray(v) ? v.filter(isString).slice(0, MAX_RECENT) : [];
  } catch {
    return [];
  }
};

// Recent emoji first, then the defaults.
export const quickReactions = (recent: string[], defaults: string[], count: number): string[] =>
  [...new Set([...recent, ...defaults])].slice(0, count);

type Point = { x: number; y: number };

// Arrow keys in the emoji grid, by the buttons' positions (sections start
// new rows). -1: up from the first row, to the search field; null: not a
// grid key.
export const gridMove = (at: Point[], from: number, key: string): number | null => {
  const last = at.length - 1;
  switch (key) {
    case "ArrowLeft":
      return Math.max(0, from - 1);
    case "ArrowRight":
      return Math.min(last, from + 1);
    case "Home":
      return 0;
    case "End":
      return last;
    case "ArrowUp":
    case "ArrowDown": {
      const here = at[from];
      if (!here) return null;
      const down = key === "ArrowDown";
      const rows = at.map((p) => p.y).filter((y) => (down ? y > here.y : y < here.y));
      if (rows.length === 0) return down ? from : -1;
      const row = down ? Math.min(...rows) : Math.max(...rows);
      const distance = (i: number) => (at[i]?.y === row ? Math.abs(at[i]!.x - here.x) : Infinity);
      return at.reduce((best, _, i) => (distance(i) < distance(best) ? i : best), from);
    }
    default:
      return null;
  }
};
