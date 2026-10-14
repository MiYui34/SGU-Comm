import { escapeHtml, type ChatMessage } from "@kallisti/protocol";

// Text channels come from the Kallisti server (JSON over WebSocket text
// frames). Against a plain Mumble server, the state stays unavailable.

export type TextChannel = { id: number; name: string };
export type TextMessage = { id: number; channel: number; author: string; text: string; at: number };

export type ServerEvent =
  | { type: "ready"; channels: TextChannel[] }
  | { type: "history"; channel: number; messages: TextMessage[]; more: boolean }
  | { type: "message"; message: TextMessage }
  | { type: "error"; message: string };

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
const isMessage = (v: unknown): v is TextMessage =>
  isObj(v) && isNum(v.id) && isNum(v.channel) && isStr(v.author) && isStr(v.text) && isNum(v.at);

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
  }
};

export const historyRequest = (channel: number, before?: number) =>
  JSON.stringify(before === undefined ? { type: "history", channel } : { type: "history", channel, before });

export const sendRequest = (channel: number, text: string) =>
  JSON.stringify({ type: "send", channel, text });

// http(s) URLs in already escaped text become links. Trailing punctuation
// and a closing bracket stay outside, like in Discord.
export const linkify = (escaped: string) =>
  escaped.replace(/https?:\/\/[^\s<]+[^\s<.,:;!?)\]'"]/g, (url) => `<a href="${url}">${url}</a>`);

// Text channel messages are plain text: escape, then link URLs.
export const toChatMessage = (m: TextMessage): ChatMessage => ({
  actor: null,
  actorName: m.author,
  channelIds: [],
  treeIds: [],
  sessions: [],
  text: linkify(escapeHtml(m.text)),
  at: m.at,
});
