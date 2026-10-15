import { proto, type ServerState } from "@kallisti/protocol";
import { certificateProblem } from "./desktop";

// Coming back by itself after a lost connection (a background tab, the
// network, a server restart): only for a connection that was in, and not
// after a kick, a refusal or a certificate question (the user decides).
export const shouldReconnect = (c: {
  status: ServerState["status"];
  closed: boolean;
  closeReason: string;
  wasSynced: boolean;
}) =>
  c.closed && c.wasSynced && c.status !== "kicked" && c.status !== "rejected" && certificateProblem(c.closeReason) === null;

const DELAYS_MS = [1000, 2000, 5000, 10_000, 20_000, 30_000];

// The wait before attempt n (0: the first).
export const reconnectDelay = (attempt: number) => DELAYS_MS[Math.min(attempt, DELAYS_MS.length - 1)]!;

// Refused because the name is in use: right after a drop, that is often our
// own old session, which the server has not let go yet. Worth a few tries.
export const busyName = (s: Pick<ServerState, "status" | "rejectType">) =>
  s.status === "rejected" && s.rejectType === proto.Reject_RejectType.UsernameInUse;
