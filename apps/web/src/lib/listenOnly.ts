import type { ServerState } from "@kallisti/protocol";

// Listen-only voice channels (built-in server): only the speakers may
// speak. Underneath, Mumble ACL entries (the server writes them).

export type Speaker = { id: number; name: string };
export type ListenOnly = { on: boolean; speakers: Speaker[] };

const byName = (a: Speaker, b: Speaker) => a.name.localeCompare(b.name);

// Who may become a speaker: registered users who are online, by name.
// SuperUser (id 0) always may.
export const speakerCandidates = (server: ServerState): Speaker[] =>
  Object.values(server.users)
    .flatMap((u) => (u.userId ? [{ id: u.userId, name: u.name }] : []))
    .sort(byName);

const speakerIds = (v: ListenOnly) => (v.on ? v.speakers.map((s) => s.id).sort((a, b) => a - b) : []);

// What to ask the server for after the dialog (null: no change).
export const listenOnlyAction = (channel: number, before: ListenOnly, after: ListenOnly) => {
  const ids = speakerIds(after);
  const same = before.on === after.on && speakerIds(before).join() === ids.join();
  return same ? null : { listenOnly: channel, on: after.on, users: ids };
};
