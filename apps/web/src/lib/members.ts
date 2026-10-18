import { byName } from "@kallisti/protocol";
import type { Peer } from "./textChannels";

// Registered users by id (built-in server, without guest accounts): the
// member list shows those who are offline, like Discord.
export type Members = Readonly<Record<number, string>>;

// Registered users who are not online (by user id: names can repeat
// between a guest and an account), by name like the online list.
export const offlineMembers = (members: Members, online: readonly { userId: number | null }[]): Peer[] => {
  const here = new Set(online.map((u) => u.userId));
  return Object.entries(members)
    .map(([id, name]) => ({ id: Number(id), name }))
    .filter((p) => !here.has(p.id))
    .sort((a, b) => byName(a.name, b.name));
};

// An offline user's bio (online ones come with their user state).
export const bioRequest = (user: number) => JSON.stringify({ type: "bio", user });
