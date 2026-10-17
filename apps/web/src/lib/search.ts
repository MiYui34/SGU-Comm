// Search filters as in Discord: "from:name" (or from:@name, from:"two
// words") and "in:#channel" (or in:"Voice channel", in:@user for a direct
// chat); the rest are words.
export type SearchQuery = { text: string; author?: string; channel?: number };

const FILTER = /(^|\s)(from|in):(?:"([^"]+)"|(\S+))/gi;

// chats: chat ids by lower-case name (text channels, voice channels,
// direct chats).
// An unknown chat is -1: the server finds nothing there.
export const parseSearch = (query: string, chats: ReadonlyMap<string, number>): SearchQuery => {
  const out: Omit<SearchQuery, "text"> = {};
  const text = query.replace(FILTER, (_, before: string, key: string, quoted?: string, word?: string) => {
    const value = quoted ?? word ?? "";
    if (key.toLowerCase() === "from") out.author = value.replace(/^@/, "");
    else out.channel = chats.get(value.replace(/^[#@]/, "").toLowerCase()) ?? -1;
    return before;
  });
  return { text: text.replace(/\s+/g, " ").trim(), ...out };
};
