// Invite links to private channels: <server>/invite/<token>. The server
// answers the path with the app, which redeems the token once connected
// (after a login if needed: the address stays until then). Staff make
// them in the members dialog; the token is shown once (only its hash is
// kept).

export type Invite = { id: number; creator: string; created: number; expires?: number; maxUses?: number; uses: number };

export const inviteLink = (origin: string, token: string) => `${origin}/invite/${token}`;

// null: not an invite link to this server (origin).
export const parseInviteLink = (url: string, origin: string): string | null => {
  let u: URL;
  try {
    u = new URL(url, origin);
  } catch {
    return null;
  }
  const m = /^\/invite\/([\w-]{20,100})$/.exec(u.pathname);
  return u.origin === new URL(origin).origin && m ? m[1]! : null;
};

// Choices when making a link (0: no end, no limit); the defaults first.
export const expiryChoices = [
  { seconds: 7 * 86400, label: "7 days" },
  { seconds: 86400, label: "1 day" },
  { seconds: 0, label: "Never" },
] as const;
export const useChoices = [
  { uses: 0, label: "No limit" },
  { uses: 1, label: "1 use" },
  { uses: 5, label: "5 uses" },
  { uses: 25, label: "25 uses" },
] as const;

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"}`;

export const inviteUses = (i: { uses: number; maxUses?: number }) =>
  i.maxUses ? `${i.uses} of ${plural(i.maxUses, "use")}` : plural(i.uses, "use");

export const inviteExpiry = (expires: number | undefined, now: number) => {
  if (expires === undefined) return "never ends";
  const minutes = Math.floor((expires - now) / 60_000);
  if (expires <= now) return "ended";
  if (minutes < 1) return "ends in under a minute";
  if (minutes < 60) return `ends in ${plural(minutes, "minute")}`;
  if (minutes < 24 * 60) return `ends in ${plural(Math.floor(minutes / 60), "hour")}`;
  return `ends in ${plural(Math.floor(minutes / (24 * 60)), "day")}`;
};

// Where a redeemed link leads: its text channel, else the voice channel.
export const invitePlace = (joined: { voice: number; channel?: number }) =>
  joined.channel !== undefined ? { kind: "text" as const, id: joined.channel } : { kind: "voice" as const, id: joined.voice };
