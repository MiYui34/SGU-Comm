import type { Registered } from "@kallisti/protocol";
import type { ChannelKind } from "./channelForm";
import type { Speaker } from "./listenOnly";

// Private channels (built-in server): only their members, admins and
// moderators see them. The server keeps them on the Mumble channel: a
// voice channel itself, or a text channel's mirror. Members are
// registered users (also guest accounts, which are registered).

export type Member = Speaker;

// The Mumble channel that holds a channel's access (null: none, a text
// channel without a mirror).
export const accessChannel = (kind: ChannelKind, id: number, textChannels: readonly { id: number; mumble?: number }[]) =>
  kind === "voice" ? id : kind === "text" ? (textChannels.find((c) => c.id === id)?.mumble ?? null) : null;

const byName = (a: Member, b: Member) => a.name.localeCompare(b.name);
const ids = (ms: readonly Member[]) => ms.map((m) => m.id).sort((a, b) => a - b);

// Who may become a member: registered users who are not members yet, by
// name. SuperUser (id 0) sees all anyway.
export const memberCandidates = (registered: readonly Registered[] | null, members: readonly Member[]): Member[] =>
  (registered ?? [])
    .filter((r) => r.userId !== 0 && !members.some((m) => m.id === r.userId))
    .map((r) => ({ id: r.userId, name: r.name }))
    .sort(byName);

// What to ask the server for after the members dialog (null: no change).
export const membersAction = (channel: number, before: readonly Member[], after: readonly Member[]) =>
  ids(before).join() === ids(after).join() ? null : { setPrivate: channel, on: true, users: ids(after) };

// What to ask for after the private switch changed (null: no change).
export const privateAction = (channel: number, before: boolean, after: boolean, members: readonly Member[]) =>
  before === after ? null : { setPrivate: channel, on: after, users: after ? ids(members) : [] };
