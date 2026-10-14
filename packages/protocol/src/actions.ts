import { encodeMessage } from "./messages";

const HTML_ESCAPES: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&#39;",
};

// Mumble text messages are HTML. Plain user input must be escaped.
export const escapeHtml = (text: string) => text.replace(/[&<>"']/g, (c) => HTML_ESCAPES[c]!);

// A channel, or one user for a private message.
export type TextTarget = { channelId: number } | { session: number };

export const textMessageInit = (target: TextTarget, text: string) => ({
  ...("channelId" in target ? { channelId: [target.channelId] } : { session: [target.session] }),
  message: escapeHtml(text),
});

export const sendText = (target: TextTarget, text: string) =>
  encodeMessage("TextMessage", textMessageInit(target, text));

export const joinChannel = (session: number, channelId: number) =>
  encodeMessage("UserState", { session, channelId });

export const setSelfState = (
  session: number,
  state: { selfMute: boolean; selfDeaf: boolean },
) => encodeMessage("UserState", { session, ...state });
