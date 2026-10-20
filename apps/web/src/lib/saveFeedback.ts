import type { ManageAction, ServerEvent } from "./textChannels";

// Feedback on a control that saves or applies something on the server:
// pending while it works, saved for a moment once it confirms, or its
// error (a refusal, or no answer in time). Settings stored on the device
// apply at once and need none.
export type Save = { phase: "idle" | "pending" | "saved" } | { phase: "error"; message: string };
export const idle: Save = { phase: "idle" };
export const pending: Save = { phase: "pending" };
// How long "Saved" shows, and how long a request waits for its answer.
export const SAVED_MS = 2_000;
export const ANSWER_MS = 5_000;

// An answer counts only while the control waits for one.
export const settle = (s: Save, error?: string): Save =>
  s.phase !== "pending" ? s : error === undefined ? { phase: "saved" } : { phase: "error", message: error };
export const fade = (s: Save): Save => (s.phase === "saved" ? idle : s);
export const busyLabel = (s: Save, label: string, busy = "Saving…") => (s.phase === "pending" ? busy : label);

// Requests that wait for the server's answer, oldest first. The server
// answers in order, so a refusal (a manage error, a Mumble
// PermissionDenied) goes to the oldest of its kind, and an event confirms
// the oldest it matches (one "channels" event is not the answer to three
// changes). Mumble checks look at the state: all that hold are done.
export type Via = "text" | "mumble";
export const answer = <W extends { via: Via }>(waiters: readonly W[], via: Via, refused: boolean, matches: (w: W) => boolean) => {
  const mine = waiters.filter((w) => w.via === via);
  const no = refused ? mine[0] : undefined;
  const matching = mine.filter((w) => w !== no && matches(w));
  const confirmed = via === "text" ? matching.slice(0, 1) : matching;
  return { refused: no, confirmed, rest: waiters.filter((w) => w !== no && !confirmed.includes(w)) };
};

const channelChanges = ["create", "createCategory", "rename", "renameCategory", "remove", "removeCategory", "order", "orderCategories", "setCategory", "topic", "slowMode"];

// The event that confirms a management request; null: the server sends
// none of its own (a ban ends the connection, deleting no messages says
// nothing).
export const confirmsManage = (a: ManageAction): ((ev: ServerEvent) => boolean) | null => {
  const is = (type: ServerEvent["type"]) => (ev: ServerEvent) => ev.type === type;
  if ("guestsReadOnly" in a) return (ev) => ev.type === "guests-read-only" && ev.on === a.guestsReadOnly;
  if ("captcha" in a) return (ev) => ev.type === "captcha" && ev.level === a.captcha;
  if ("setMessageRules" in a || "messageRules" in a) return is("message-rules");
  if ("setPrivate" in a || "createPrivateVoice" in a) return is("private");
  if ("invites" in a || "deleteInvite" in a) return is("invites");
  if ("createInvite" in a) return is("invite-created");
  if ("redeemInvite" in a) return is("invite-joined");
  if ("moderator" in a || "admin" in a || "channelModerator" in a) return is("roles");
  if ("listenOnly" in a) return is("listen-only");
  if ("timeout" in a || "channelTimeout" in a) return is("timeouts");
  if ("connections" in a) return is("connections");
  if ("recordings" in a) return is("recordings");
  if ("record" in a) return is("recording");
  if (channelChanges.some((k) => k in a)) return is("channels");
  return null;
};
