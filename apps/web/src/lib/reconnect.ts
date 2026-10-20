import { proto, type ServerState } from "@kallisti/protocol";
import { certificateProblem } from "./desktop";
import type { Place } from "./place";

// The server closed the connection because its session ended (a logout,
// also on another device, or a password change there). Same reason text as
// the bridge's Revoke.
export const loggedOut = (closeReason: string) => closeReason === "logged out";

// Coming back by itself after a lost connection (a background tab, the
// network, a server restart): only for a connection that was in, and not
// after a kick, a refusal, a logout or a certificate question (the user
// decides).
export const shouldReconnect = (c: {
  status: ServerState["status"];
  closed: boolean;
  closeReason: string;
  wasSynced: boolean;
}) =>
  c.closed && c.wasSynced && c.status !== "kicked" && c.status !== "rejected" &&
  !loggedOut(c.closeReason) &&
  certificateProblem(c.closeReason) === null;

const DELAYS_MS = [1000, 2000, 5000, 10_000, 20_000, 30_000];

// The wait before attempt n (0: the first).
export const reconnectDelay = (attempt: number) => DELAYS_MS[Math.min(attempt, DELAYS_MS.length - 1)]!;

// Refused because the name is in use: right after a drop, that is often our
// own old session, which the server has not let go yet. Worth a few tries.
export const busyName = (s: Pick<ServerState, "status" | "rejectType">) =>
  s.status === "rejected" && s.rejectType === proto.Reject_RejectType.UsernameInUse;

// After a reconnect: the voice channel to join again (null: none, already
// there, or the channel is gone). voiceOther: our account's session in
// voice now, if not this one. That may be our old session, which the
// server has not let go yet; another device that went into voice meanwhile
// keeps it (the user can still take voice with a click).
export const rejoinTarget = (back: Place | null, here: number, exists: (id: number) => boolean, voiceOther: number | null): number | null =>
  back?.voice != null && back.voice !== here && exists(back.voice) && (voiceOther === null || voiceOther === back.session) ? back.voice : null;
