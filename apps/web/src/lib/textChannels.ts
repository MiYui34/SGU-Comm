import { escapeHtml, type ChatMessage } from "@kallisti/protocol";
import { markdown } from "./markdown";
import type { Reaction } from "./reactions";
import type { LinkPreview } from "./linkPreview";
import type { Roles } from "./roles";

// Text channels come from the Kallisti server (JSON over WebSocket text
// frames). Against a plain Mumble server, the state stays unavailable.

// mumble: the Mumble channel that mirrors it for Mumble clients.
export type TextChannel = { id: number; name: string; mumble?: number; topic?: string };
export type Attachment = { url: string; name: string; type: string; size: number };
export type TextMessage = {
  id: number;
  channel: number;
  author: string;
  text: string;
  at: number;
  // The author was registered on the Mumble server (a verified name).
  verified?: boolean;
  upload?: Attachment;
  edited?: number; // Unix milliseconds
  // The message it answers (author empty: deleted since).
  reply?: Quote;
  reactions?: Reaction[];
};
export type Quote = { id: number; author: string; text: string };
// A chat line for display: a text channel message can carry an attachment
// and a verified mark.
// source: the stored message (id and plain text), for editing.
export type DisplayMessage = ChatMessage & {
  upload?: Attachment;
  verified?: boolean;
  edited?: number;
  source?: { id: number; text: string };
  reply?: Quote;
  reactions?: Reaction[];
};

export type SummaryEntry = { channel: number; voice?: number; latest: number; mentions: number };

export type ServerEvent =
  // voiceChats: voice channels have stored chats (the built-in server).
  | {
      type: "ready";
      channels: TextChannel[];
      voiceChats?: boolean;
      roles?: Roles;
      // Registered users: read marks, and per channel the newest message
      // and unread mentions (voice: the voice channel of a voice chat).
      reads?: Record<number, number>;
      summary?: SummaryEntry[];
    }
  | ({ type: "roles" } & Roles)
  // The channel that holds a voice channel's chat.
  | { type: "voice-chat"; voice: number; channel: number }
  // The text channels changed (an admin managed them).
  | { type: "channels"; channels: TextChannel[] }
  | { type: "history"; channel: number; messages: TextMessage[]; more: boolean }
  | { type: "message"; message: TextMessage }
  | { type: "message-edited"; message: TextMessage }
  | { type: "message-deleted"; channel: number; id: number }
  | { type: "search-results"; text: string; messages: TextMessage[] }
  // about "manage": the answer to a channel or role change.
  | { type: "error"; message: string; about?: "manage" }
  | { type: "upload-token"; token: string }
  // A message's reactions changed (all of them).
  | { type: "reactions"; channel: number; message: number; reactions: Reaction[] }
  // A message's link preview (null: none).
  | { type: "preview"; message: number; preview: LinkPreview | null }
  // Someone types in a channel (see lib/typing).
  | { type: "typing"; channel: number; name: string }
  // The server dropped messages for us: reload what is shown.
  | { type: "resync" };

export type TextState = {
  available: boolean;
  channels: TextChannel[];
  voiceChats: boolean;
  // Voice channel id -> the channel that holds its chat.
  voiceChannels: Readonly<Record<number, number>>;
  messages: Readonly<Record<number, TextMessage[]>>; // per channel, by id
  loaded: Readonly<Record<number, boolean>>;
  more: Readonly<Record<number, boolean>>;
  lastError: string | null;
  roles: Roles;
  // From ready (registered users): read marks, and the newest message and
  // unread mentions per channel at that time.
  reads: Readonly<Record<number, number>>;
  summary: Readonly<Record<number, { latest: number; mentions: number }>>;
  // Errors about channel and role changes show as a notice (n counts them), not in
  // the chat.
  notice: { text: string; n: number } | null;
  // The latest search answer.
  search: { text: string; messages: TextMessage[] } | null;
  // Link previews by message (null: none).
  previews: Readonly<Record<number, LinkPreview | null>>;
  // Counts resync events, so views can reload.
  resyncs: number;
};

export const initialTextState: TextState = {
  available: false,
  channels: [],
  voiceChats: false,
  roles: { admins: [], moderators: [] },
  reads: {},
  summary: {},
  voiceChannels: {},
  messages: {},
  loaded: {},
  more: {},
  lastError: null,
  notice: null,
  search: null,
  previews: {},
  resyncs: 0,
};

const isNum = (v: unknown): v is number => typeof v === "number" && Number.isFinite(v);
const isSummaryEntry = (v: unknown): v is SummaryEntry =>
  typeof v === "object" &&
  v !== null &&
  isNum((v as SummaryEntry).channel) &&
  isNum((v as SummaryEntry).latest) &&
  isNum((v as SummaryEntry).mentions) &&
  ((v as SummaryEntry).voice === undefined || isNum((v as SummaryEntry).voice));
const isIds = (v: unknown): v is number[] => Array.isArray(v) && v.every(isNum);
const parseRoles = (v: unknown): Roles | null =>
  typeof v === "object" && v !== null && isIds((v as Roles).admins) && isIds((v as Roles).moderators)
    ? { admins: (v as Roles).admins, moderators: (v as Roles).moderators }
    : null;

const isStr = (v: unknown): v is string => typeof v === "string";
const isObj = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object";

const isChannel = (v: unknown): v is TextChannel =>
  isObj(v) &&
  isNum(v.id) &&
  isStr(v.name) &&
  (v.mumble === undefined || isNum(v.mumble)) &&
  (v.topic === undefined || typeof v.topic === "string");
const isAttachment = (v: unknown): v is Attachment =>
  isObj(v) && isStr(v.url) && isStr(v.name) && isStr(v.type) && isNum(v.size);
const optStr = (v: unknown) => v === undefined || isStr(v);
const isPreview = (v: unknown): v is LinkPreview =>
  isObj(v) &&
  isStr(v.url) &&
  /^https?:\/\//.test(v.url) &&
  isStr(v.title) &&
  optStr(v.description) &&
  optStr(v.site) &&
  (v.image === undefined || (isStr(v.image) && v.image.startsWith("/api/link-image/")));

const isReactions = (v: unknown): v is Reaction[] =>
  Array.isArray(v) && v.every((r) => isObj(r) && isStr(r.emoji) && Array.isArray(r.users) && r.users.every(isStr));

const isMessage = (v: unknown): v is TextMessage =>
  isObj(v) &&
  isNum(v.id) &&
  isNum(v.channel) &&
  isStr(v.author) &&
  isStr(v.text) &&
  isNum(v.at) &&
  (v.verified === undefined || typeof v.verified === "boolean") &&
  (v.edited === undefined || isNum(v.edited)) &&
  (v.reply === undefined || (isObj(v.reply) && isNum(v.reply.id) && isStr(v.reply.author) && isStr(v.reply.text))) &&
  (v.upload === undefined || isAttachment(v.upload)) &&
  (v.reactions === undefined || isReactions(v.reactions));

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
    case "ready": {
      if (!Array.isArray(v.channels) || !v.channels.every(isChannel)) return null;
      const roles = parseRoles(v.roles);
      const reads = isObj(v.reads) && Object.values(v.reads).every(isNum) ? (v.reads as Record<number, number>) : undefined;
      const summary = Array.isArray(v.summary) ? v.summary.filter(isSummaryEntry) : undefined;
      return {
        type: "ready",
        channels: v.channels,
        ...(v.voiceChats === true ? { voiceChats: true } : {}),
        ...(roles ? { roles } : {}),
        ...(reads ? { reads: Object.fromEntries(Object.entries(reads).map(([k, n]) => [Number(k), n])) } : {}),
        ...(summary ? { summary } : {}),
      };
    }
    case "roles": {
      const roles = parseRoles(v);
      return roles && { type: "roles", ...roles };
    }
    case "voice-chat":
      return isNum(v.voice) && isNum(v.channel) ? { type: "voice-chat", voice: v.voice, channel: v.channel } : null;
    case "channels":
      return Array.isArray(v.channels) && v.channels.every(isChannel) ? { type: "channels", channels: v.channels } : null;
    case "history":
      return isNum(v.channel) && Array.isArray(v.messages) && v.messages.every(isMessage)
        ? { type: "history", channel: v.channel, messages: v.messages, more: v.more === true }
        : null;
    case "message":
    case "message-edited":
      return isMessage(v.message) ? { type: v.type, message: v.message } : null;
    case "message-deleted":
      return isNum(v.channel) && isNum(v.id) ? { type: "message-deleted", channel: v.channel, id: v.id } : null;
    case "search-results":
      return isStr(v.text) && Array.isArray(v.messages) && v.messages.every(isMessage)
        ? { type: "search-results", text: v.text, messages: v.messages }
        : null;
    case "error":
      if (!isStr(v.message)) return null;
      return v.about === "manage" ? { type: "error", message: v.message, about: "manage" } : { type: "error", message: v.message };
    case "upload-token":
      return isStr(v.token) ? { type: "upload-token", token: v.token } : null;
    case "resync":
      return { type: "resync" };
    case "typing":
      return isNum(v.channel) && isStr(v.name) ? { type: "typing", channel: v.channel, name: v.name } : null;
    case "preview":
      if (!isNum(v.message)) return null;
      if (v.preview === null) return { type: "preview", message: v.message, preview: null };
      return isPreview(v.preview) ? { type: "preview", message: v.message, preview: v.preview } : null;
    case "reactions":
      return isNum(v.channel) && isNum(v.message) && isReactions(v.reactions)
        ? { type: "reactions", channel: v.channel, message: v.message, reactions: v.reactions }
        : null;
    default:
      return null;
  }
};

const merge = (a: readonly TextMessage[] = [], b: readonly TextMessage[]) =>
  [...new Map([...a, ...b].map((m) => [m.id, m])).values()].sort((x, y) => x.id - y.id);

const patchSearch = (search: TextState["search"], f: (ms: TextMessage[]) => TextMessage[]) =>
  search && { ...search, messages: f(search.messages) };

// A search is on its way: the old results go.
export const startSearch = (state: TextState): TextState => ({ ...state, search: null });

export const reduceText = (state: TextState, event: ServerEvent): TextState => {
  switch (event.type) {
    case "ready":
      return {
        ...state,
        available: true,
        channels: event.channels,
        voiceChats: event.voiceChats === true,
        roles: event.roles ?? initialTextState.roles,
        reads: event.reads ?? {},
        summary: Object.fromEntries((event.summary ?? []).map((e) => [e.channel, { latest: e.latest, mentions: e.mentions }])),
        voiceChannels: {
          ...state.voiceChannels,
          ...Object.fromEntries((event.summary ?? []).flatMap((e) => (e.voice === undefined ? [] : [[e.voice, e.channel]]))),
        },
      };
    case "roles":
      return { ...state, roles: { admins: event.admins, moderators: event.moderators } };
    case "voice-chat":
      return { ...state, voiceChannels: { ...state.voiceChannels, [event.voice]: event.channel } };
    case "channels":
      return { ...state, channels: event.channels };
    case "history":
      return {
        ...state,
        lastError: null,
        messages: { ...state.messages, [event.channel]: merge(state.messages[event.channel], event.messages) },
        loaded: { ...state.loaded, [event.channel]: true },
        more: { ...state.more, [event.channel]: event.more },
      };
    case "message":
    case "message-edited": {
      const ch = event.message.channel;
      const search = event.type === "message-edited" ? patchSearch(state.search, (ms) => ms.map((m) => (m.id === event.message.id ? event.message : m))) : state.search;
      // An edit of a message not loaded here stays unloaded.
      if (event.type === "message-edited" && !state.messages[ch]?.some((m) => m.id === event.message.id)) return { ...state, search };
      // An edit may change the link: its preview is asked again.
      const { [event.message.id]: _, ...previews } = state.previews;
      return { ...state, search, previews, messages: { ...state.messages, [ch]: merge(state.messages[ch], [event.message]) } };
    }
    case "search-results":
      return { ...state, search: { text: event.text, messages: event.messages } };
    case "preview":
      return { ...state, previews: { ...state.previews, [event.message]: event.preview } };
    case "reactions": {
      const msgs = state.messages[event.channel];
      const i = msgs?.findIndex((m) => m.id === event.message) ?? -1;
      if (!msgs || i < 0) return state;
      const next = msgs.slice();
      next[i] = { ...msgs[i]!, reactions: event.reactions };
      return { ...state, messages: { ...state.messages, [event.channel]: next } };
    }
    case "message-deleted": {
      const search = patchSearch(state.search, (ms) => ms.filter((m) => m.id !== event.id));
      const msgs = state.messages[event.channel];
      if (!msgs) return { ...state, search };
      return { ...state, search, messages: { ...state.messages, [event.channel]: msgs.filter((m) => m.id !== event.id) } };
    }
    case "error":
      return event.about === "manage"
        ? { ...state, notice: { text: event.message, n: (state.notice?.n ?? 0) + 1 } }
        : { ...state, lastError: event.message };
    case "upload-token": // answered by the connection, not state
    case "typing": // short-lived: the client keeps it (lib/typing)
      return state;
    case "resync":
      return { ...state, messages: {}, loaded: {}, more: {}, resyncs: state.resyncs + 1 };
  }
};

export const historyRequest = (channel: number, before?: number) =>
  JSON.stringify(before === undefined ? { type: "history", channel } : { type: "history", channel, before });

export const sendRequest = (channel: number, text: string, upload?: string, reply?: number) =>
  JSON.stringify({ type: "send", channel, text, ...(upload ? { upload } : {}), ...(reply ? { reply } : {}) });

export const uploadTokenRequest = () => JSON.stringify({ type: "upload-token" });

export const voiceChatRequest = (voice: number) => JSON.stringify({ type: "voice-chat", voice });

// Text channel management (admins; the server checks).
export type ManageAction =
  | { create: string }
  | { rename: number; name: string }
  | { remove: number }
  | { order: number[] }
  | { moderator: number; on: boolean }
  | { topic: number; text: string };

export const manageRequest = (a: ManageAction) =>
  JSON.stringify(
    "create" in a
      ? { type: "create-channel", name: a.create }
      : "rename" in a
        ? { type: "rename-channel", channel: a.rename, name: a.name }
        : "remove" in a
          ? { type: "delete-channel", channel: a.remove }
          : "moderator" in a
            ? { type: "set-moderator", user: a.moderator, moderator: a.on }
            : "topic" in a
              ? { type: "set-topic", channel: a.topic, text: a.text }
              : { type: "order-channels", channels: a.order },
  );

export const searchRequest = (text: string) => JSON.stringify({ type: "search", text });
export const typingRequest = (channel: number) => JSON.stringify({ type: "typing", channel });
export const markReadRequest = (channel: number, message: number) =>
  JSON.stringify({ type: "mark-read", channel, message });
export const editMessageRequest = (message: number, text: string) =>
  JSON.stringify({ type: "edit-message", message, text });
export const deleteMessageRequest = (message: number) => JSON.stringify({ type: "delete-message", message });

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

export { linkify } from "./markdown";

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

// Text channel messages are plain text: escape, then Markdown. Attachment
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
      text: markdown(escapeHtml(m.text)),
      at: m.at,
      peer: null,
      verified: m.verified === true,
      source: { id: m.id, text: m.text },
      ...(m.edited !== undefined && { edited: m.edited }),
      ...(m.reply && { reply: m.reply }),
      ...(m.reactions?.length && { reactions: m.reactions }),
      ...(m.upload && url && { upload: { ...m.upload, url } }),
    };
    chatCache.set(m, chat);
  }
  return chat;
};

// Channels with a message newer than the newest one seen there (loaded,
// or known from the server's summary).
export const unreadTextIds = (state: TextState, seen: Record<number, number>): Set<number> =>
  new Set(
    state.channels
      .filter((c) => Math.max(state.messages[c.id]?.at(-1)?.id ?? 0, state.summary[c.id]?.latest ?? 0) > (seen[c.id] ?? 0))
      .map((c) => c.id),
  );
