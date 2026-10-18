// Message rules (Server settings → Message rules, admins): blocked words
// and link domains, and limits for new accounts and guests. The server
// checks every message (staff are exempt) and says why it refuses one;
// only admins get the rules.
export type MessageRules = {
  newMinutes: number; // accounts younger than this, and guests, are limited (0: off)
  words: string[]; // words and phrases, matched as whole words in any case
  domains: string[]; // with their subdomains
};

export const MAX_NEW_MINUTES = 7 * 24 * 60;

const isStrings = (v: unknown): v is string[] => Array.isArray(v) && v.every((s) => typeof s === "string");

export const parseMessageRules = (v: unknown): MessageRules | null => {
  if (!v || typeof v !== "object") return null;
  const r = v as Record<string, unknown>;
  return typeof r.newMinutes === "number" && isStrings(r.words) && isStrings(r.domains)
    ? { newMinutes: r.newMinutes, words: r.words, domains: r.domains }
    : null;
};

// The lists are edited one entry per line.
export const parseList = (text: string) =>
  text
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
export const formatList = (list: readonly string[]) => list.join("\n");

export const minutesError = (text: string) => {
  const n = Number(text.trim());
  return text.trim() !== "" && Number.isInteger(n) && n >= 0 && n <= MAX_NEW_MINUTES
    ? null
    : `Minutes: a whole number from 0 (off) to ${MAX_NEW_MINUTES}.`;
};

// The form's fields as rules (the server checks and tidies them again).
export const rulesFrom = (minutes: string, words: string, domains: string): MessageRules => ({
  newMinutes: Number(minutes.trim()),
  words: parseList(words),
  domains: parseList(domains),
});
