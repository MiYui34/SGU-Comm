import { byName } from "@kallisti/protocol";
import type { Peer } from "./textChannels";

// Registered users by id (built-in server, without guest accounts): the
// member list shows those who are offline, like Discord.
export type Members = Readonly<Record<number, string>>;
// Members online in any channel: their session by user id. Those in
// channels we cannot see are online without a channel, like Discord.
export type Online = Readonly<Record<number, number>>;
export type OnlineMember = Peer & { session: number };

type Seen = readonly { userId: number | null }[];
const peers = (members: Members) => Object.entries(members).map(([id, name]) => ({ id: Number(id), name }));
const seenIds = (users: Seen) => new Set(users.map((u) => u.userId));

// Registered users who are not online (by user id: names can repeat
// between a guest and an account), by name like the online list.
export const offlineMembers = (members: Members, online: Online, users: Seen): Peer[] => {
  const here = seenIds(users);
  return peers(members)
    .filter((p) => !here.has(p.id) && online[p.id] === undefined)
    .sort((a, b) => byName(a.name, b.name));
};

// Members online whom we do not see (their channel is hidden from us, or
// their user state has not come yet), by name.
export const hiddenMembers = (members: Members, online: Online, users: Seen): OnlineMember[] => {
  const here = seenIds(users);
  return peers(members)
    .flatMap((p) => (here.has(p.id) || online[p.id] === undefined ? [] : [{ ...p, session: online[p.id]! }]))
    .sort((a, b) => byName(a.name, b.name));
};

// The online group: users we see and hidden members, by name.
export type OnlineRow<U> = { user: U } | { member: OnlineMember };
export const onlineRows = <U extends { name: string }>(users: readonly U[], hidden: readonly OnlineMember[]): OnlineRow<U>[] => {
  const name = (r: OnlineRow<U>) => ("user" in r ? r.user.name : r.member.name);
  return [...users.map((user) => ({ user })), ...hidden.map((member) => ({ member }))].sort((a, b) => byName(name(a), name(b)));
};

// An offline user's bio (online ones come with their user state).
export const bioRequest = (user: number) => JSON.stringify({ type: "bio", user });
