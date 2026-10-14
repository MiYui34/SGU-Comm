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

export const sendText = (channelId: number, text: string) =>
  encodeMessage("TextMessage", { channelId: [channelId], message: escapeHtml(text) });

export const joinChannel = (session: number, channelId: number) =>
  encodeMessage("UserState", { session, channelId });

export const setSelfState = (
  session: number,
  state: { selfMute: boolean; selfDeaf: boolean },
) => encodeMessage("UserState", { session, ...state });
