import { escapeHtml, type ChatMessage } from "@kallisti/protocol";

// Text channels come from the Kallisti server (JSON over WebSocket text
// frames). Against a plain Mumble server, the state stays unavailable.

export type TextChannel = { id: number; name: string };
export type Attachment = { url: string; name: string; type: string; size: number };
export type TextMessage = {
  id: number;
  channel: number;
  author: string;
  text: string;
  at: number;
  upload?: Attachment;
};
// A chat line for display: a text channel message can carry an attachment.
export type DisplayMessage = ChatMessage & { upload?: Attachment };

export type ServerEvent =
  | { type: "ready"; channels: TextChannel[] }
  | { type: "history"; channel: number; messages: TextMessage[]; more: boolean }
  | { type: "message"; message: TextMessage }
  | { type: "error"; message: string }
  | { type: "upload-token"; token: string };

export type TextState = {
  available: boolean;
  channels: TextChannel[];
  messages: Readonly<Record<number, TextMessage[]>>; // per channel, by id
  loaded: Readonly<Record<number, boolean>>;
  more: Readonly<Record<number, boolean>>;
  lastError: string | null;
};

export const initialTextState: TextState = {
  available: false,
  channels: [],
  messages: {},
  loaded: {},
  more: {},
  lastError: null,
};

const isNum = (v: unknown): v is number => typeof v === "number" && Number.isFinite(v);
const isStr = (v: unknown): v is string => typeof v === "string";
const isObj = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object";

const isChannel = (v: unknown): v is TextChannel => isObj(v) && isNum(v.id) && isStr(v.name);
const isAttachment = (v: unknown): v is Attachment =>
  isObj(v) && isStr(v.url) && isStr(v.name) && isStr(v.type) && isNum(v.size);
const isMessage = (v: unknown): v is TextMessage =>
  isObj(v) &&
  isNum(v.id) &&
  isNum(v.channel) &&
  isStr(v.author) &&
  isStr(v.text) &&
  isNum(v.at) &&
  (v.upload === undefined || isAttachment(v.upload));

// Network input: anything unexpected is dropped.
export const parseServerEvent = (raw: string): ServerEvent | null => {
  let v: unknown;
  try {
    v = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!isObj(v)) return null;
  switch (v.type) {
    case "ready":
      return Array.isArray(v.channels) && v.channels.every(isChannel)
        ? { type: "ready", channels: v.channels }
        : null;
    case "history":
      return isNum(v.channel) && Array.isArray(v.messages) && v.messages.every(isMessage)
        ? { type: "history", channel: v.channel, messages: v.messages, more: v.more === true }
        : null;
    case "message":
      return isMessage(v.message) ? { type: "message", message: v.message } : null;
    case "error":
      return isStr(v.message) ? { type: "error", message: v.message } : null;
    case "upload-token":
      return isStr(v.token) ? { type: "upload-token", token: v.token } : null;
    default:
      return null;
  }
};

const merge = (a: readonly TextMessage[] = [], b: readonly TextMessage[]) =>
  [...new Map([...a, ...b].map((m) => [m.id, m])).values()].sort((x, y) => x.id - y.id);

export const reduceText = (state: TextState, event: ServerEvent): TextState => {
  switch (event.type) {
    case "ready":
      return { ...state, available: true, channels: event.channels };
    case "history":
      return {
        ...state,
        lastError: null,
        messages: { ...state.messages, [event.channel]: merge(state.messages[event.channel], event.messages) },
        loaded: { ...state.loaded, [event.channel]: true },
        more: { ...state.more, [event.channel]: event.more },
      };
    case "message": {
      const ch = event.message.channel;
      return { ...state, messages: { ...state.messages, [ch]: merge(state.messages[ch], [event.message]) } };
    }
    case "error":
      return { ...state, lastError: event.message };
    case "upload-token": // answered by the connection, not state
      return state;
  }
};

export const historyRequest = (channel: number, before?: number) =>
  JSON.stringify(before === undefined ? { type: "history", channel } : { type: "history", channel, before });

export const sendRequest = (channel: number, text: string, upload?: string) =>
  JSON.stringify(upload ? { type: "send", channel, text, upload } : { type: "send", channel, text });

export const uploadTokenRequest = () => JSON.stringify({ type: "upload-token" });

// The server's HTTP origin, from the bridge WebSocket URL.
export const httpBase = (wsUrl: string) => {
  const u = new URL(wsUrl);
  u.protocol = u.protocol === "wss:" ? "https:" : "http:";
  return u.origin;
};

export const formatSize = (bytes: number) => {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
};

// http(s) URLs in already escaped text become links. Trailing punctuation
// and a closing bracket stay outside, like in Discord.
export const linkify = (escaped: string) =>
  escaped.replace(/https?:\/\/[^\s<]+[^\s<.,:;!?)\]'"]/g, (url) => `<a href="${url}">${url}</a>`);

// Cached per message object: the reducer keeps message objects, so the chat
// list gets stable objects and does not re-render every line.
// An attachment URL from the server must stay on the server, under /files/.
// Anything else is dropped: an image would load by itself (and leak the
// reader's IP to another host).
const fileUrl = (url: string, base: string): string | null => {
  try {
    const u = new URL(url, base);
    return u.origin === new URL(base).origin && u.pathname.startsWith("/files/") ? u.href : null;
  } catch {
    return null;
  }
};

const chatCache = new WeakMap<TextMessage, DisplayMessage>();

// Text channel messages are plain text: escape, then link URLs. Attachment
// URLs are relative to the server (`base`).
export const toChatMessage = (m: TextMessage, base = ""): DisplayMessage => {
  let chat = chatCache.get(m);
  if (!chat) {
    const url = m.upload && fileUrl(m.upload.url, base);
    chat = {
      actor: null,
      actorName: m.author,
      channelIds: [],
      treeIds: [],
      sessions: [],
      text: linkify(escapeHtml(m.text)),
      at: m.at,
      ...(m.upload && url && { upload: { ...m.upload, url } }),
    };
    chatCache.set(m, chat);
  }
  return chat;
};

// Channels with a message newer than the newest one seen there.
export const unreadTextIds = (state: TextState, seen: Record<number, number>): Set<number> =>
  new Set(
    state.channels
      .filter((c) => (state.messages[c.id]?.at(-1)?.id ?? 0) > (seen[c.id] ?? 0))
      .map((c) => c.id),
  );
