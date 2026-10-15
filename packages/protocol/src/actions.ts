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

export const requestDescriptions = (channelIds: number[]) =>
  encodeMessage("RequestBlob", { channelDescription: channelIds });

// Registers the own user on the server (needs a client certificate; the
// Kallisti server provides one for accounts).
export const registerSelf = (session: number) => encodeMessage("UserState", { session, userId: 0 });

// Channel management (the server checks the permissions).
export const createChannel = (parent: number, name: string, opts: { temporary?: boolean } = {}) =>
  encodeMessage("ChannelState", { parent, name, temporary: opts.temporary ?? false });

export const editChannel = (
  channelId: number,
  fields: { name?: string; description?: string; position?: number },
) => encodeMessage("ChannelState", { channelId, ...fields });

export const removeChannel = (channelId: number) => encodeMessage("ChannelRemove", { channelId });

// What the own user may do in a channel; the answer is a PermissionQuery.
export const queryPermissions = (channelId: number) => encodeMessage("PermissionQuery", { channelId });
