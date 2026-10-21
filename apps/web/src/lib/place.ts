import { safeGet, safeSet } from "./storage";

// What the main pane shows. A direct message keeps the partner's name, so
// it still shows after they left; session: null when they are offline;
// userId: registered partners (stored direct messages).
export type View =
  | { kind: "voice" }
  | { kind: "text"; id: number }
  | { kind: "dm"; session: number | null; name: string; userId?: number | null };

// Where the user is, to come back to after a reconnect or a refresh: our
// session, the view, the voice channel (null: not in voice), the open
// thread and the self-mute state (a new session starts unmuted: it must
// not come back with a live microphone).
export type Place = { session: number | null; view: View; voice: number | null; thread: number | null; selfMute: boolean; selfDeaf: boolean };

// Kept per tab (sessionStorage) across a refresh, per site, server and user.
type Storage = Parameters<typeof safeGet>[0];
export const placeKey = (site: string, server: string, username: string) => `place:${JSON.stringify([site, server, username])}`;

// left: saved as the page goes (pagehide): a refresh. A copied tab copies
// the storage while this page is still open, so its copy lacks it.
export const savePlace = (storage: Storage, key: string, place: Place | null, left = false) =>
  safeSet(storage, key, place ? JSON.stringify({ ...place, left }) : "");

const isId = (x: unknown) => typeof x === "number" && Number.isInteger(x);
const isIdOrNull = (x: unknown) => x === null || isId(x);
const isView = (v: Partial<Record<string, unknown>>): v is View =>
  v.kind === "voice" ||
  (v.kind === "text" && isId(v.id)) ||
  (v.kind === "dm" && isIdOrNull(v.session) && typeof v.name === "string" && (v.userId === undefined || isIdOrNull(v.userId)));

// The saved place. Its session only if the page left (a refresh: our old
// session may still be in voice for a moment, see rejoinTarget); in a
// copied tab it belongs to the other tab. null: nothing usable saved.
export const loadPlace = (storage: Storage, key: string): Place | null => {
  try {
    const p = JSON.parse(safeGet(storage, key) ?? "");
    const ok =
      p && typeof p === "object" && p.view && typeof p.view === "object" && isView(p.view) &&
      isIdOrNull(p.voice) && isIdOrNull(p.thread) && typeof p.selfMute === "boolean" && typeof p.selfDeaf === "boolean";
    return ok ? { session: p.left === true && isId(p.session) ? p.session : null, view: p.view, voice: p.voice, thread: p.thread, selfMute: p.selfMute, selfDeaf: p.selfDeaf } : null;
  } catch {
    return null;
  }
};

// A direct chat with the partner's session now (null: offline): by user id
// for registered partners, else by name (a guest).
export const relinkDirect = (view: View, users: { session: number; name: string; userId?: number | null }[]): View => {
  if (view.kind !== "dm") return view;
  const user = users.find((u) => (view.userId != null ? u.userId === view.userId : u.userId == null && u.name === view.name));
  return { ...view, session: user?.session ?? null };
};
