import { escapeHtml, type ChatMessage } from "@kallisti/protocol";
import { markdown } from "./markdown";
import { spoilerUpload } from "./media";
import type { Reaction } from "./reactions";
import type { LinkPreview } from "./linkPreview";
import type { Speaker } from "./listenOnly";
import type { Roles } from "./roles";
import { isMentionAll, type MentionAll } from "./mentions";
import type { SearchQuery } from "./search";
import { parsePresence, type PresenceView } from "./presence";
import { parseCustomEmoji, type CustomEmoji } from "./customEmoji";
import type { Invite } from "./invites";
import type { Members, Online } from "./members";
import { parseConnections, type Connection } from "./connections";
import { isCaptchaLevel, type CaptchaLevel } from "./captcha";
import { parseMessageRules, type MessageRules } from "./messageRules";
import { parseRecordings, type FinishedRecording } from "./recording";

// Text channels come from the Kallisti server (JSON over WebSocket text
// frames). Against a plain Mumble server, the state stays unavailable.

// mumble: the Mumble channel that mirrors it for Mumble clients;
// slowMode: seconds between a user's posts (see lib/slowMode).
export type TextChannel = { id: number; name: string; mumble?: number; topic?: string; slowMode?: number };
export type Attachment = { url: string; name: string; type: string; size: number };
export type TextMessage = {
  id: number;
  channel: number;
  author: string;
  text: string;
  at: number;
  // The author was registered on the Mumble server (a verified name),
  // with this user id (unknown for old messages).
  verified?: boolean;
  authorId?: number;
  upload?: Attachment;
  edited?: number; // Unix milliseconds
  pinned?: number; // Unix milliseconds
  // The message it answers (author empty: deleted since).
  reply?: Quote;
  reactions?: Reaction[];
  mentionAll?: MentionAll;
  noPing?: boolean; // a reply that does not ping
};
export type Quote = { id: number; author: string; text: string };
// A chat line for display: a text channel message can carry an attachment
// and a verified mark.
// source: the stored message (id and plain text), for editing.
export type DisplayMessage = ChatMessage & {
  upload?: Attachment & { spoiler?: true };
  verified?: boolean;
  authorId?: number;
  edited?: number;
  pinned?: number;
  source?: { id: number; text: string };
  reply?: Quote;
  reactions?: Reaction[];
  mentionAll?: MentionAll;
  noPing?: boolean; // a reply that does not ping
};

export type ListenOnlyChannel = { channel: number; speakers: Speaker[] };
// A private channel (by Mumble channel) and, for staff, its members.
export type PrivateChannel = { channel: number; members?: Speaker[] };
// The other user of a direct chat.
export type Peer = { id: number; name: string };
export type DirectChat = { channel: number; peer: Peer };
// A screen share in a voice channel, by its streamer's session.
export type LiveStream = { session: number; channel: number; preview?: string };

export type SessionStatus = PresenceView & { session: number };
// A running time-out (built-in server): of a registered user (user id) or
// a guest (session); until: Unix milliseconds.
export type Timeout = { user?: number; session?: number; until: number };

export type SummaryEntry = { channel: number; voice?: number; latest: number; mentions: number };

export type ServerEvent =
  // voiceChats: voice channels have stored chats (the built-in server).
  | {
      type: "ready";
      channels: TextChannel[];
      voiceChats?: boolean;
      uploadMax?: number; // bytes per upload
      roles?: Roles;
      listenOnly?: ListenOnlyChannel[];
      private?: PrivateChannel[];
      timeouts?: Timeout[];
      // Guests may only read (see lib/guests).
      guestsReadOnly?: boolean;
      // The browser check for new accounts (see lib/captcha).
      captcha?: CaptchaLevel;
      // Registered users' direct chats (with messages).
      directs?: DirectChat[];
      streams?: LiveStream[];
      // The recorded voice channels (absent: the server does not record).
      recording?: number[];
      // Registered users: read marks, and per channel the newest message
      // and unread mentions (voice: the voice channel of a voice chat).
      reads?: Record<number, number>;
      summary?: SummaryEntry[];
      // Web clients that are not plainly online, and our chosen status.
      statuses?: SessionStatus[];
      ownStatus?: PresenceView;
      emoji?: CustomEmoji[];
      // Registered users but guest accounts (see lib/members).
      members?: Peer[];
      online?: Online;
    }
  // A registered user was added or renamed (name), or removed (gone).
  | { type: "member"; id: number; name?: string }
  // A member came online in any channel (session), or went offline.
  | { type: "online"; id: number; session?: number }
  // A registered user's saved bio (asked for, see lib/members).
  | { type: "bio"; user: number; text: string }
  // A web client's status (see lib/presence); plain online: none.
  | ({ type: "status" } & SessionStatus)
  // Our chosen status changed (here or on another device).
  | ({ type: "own-status" } & PresenceView)
  // The custom emoji changed.
  | { type: "emoji"; emoji: CustomEmoji[] }
  | ({ type: "roles" } & Roles)
  // A direct chat with a user (asked for, or a new one's first message).
  | ({ type: "dm" } & DirectChat)
  // The live screen shares changed.
  | { type: "streams"; streams: LiveStream[] }
  // A screen share's new thumbnail (a JPEG data URL).
  | { type: "stream-preview"; session: number; image: string }
  // A token for the stream connection (asked for by the connection).
  | { type: "stream-token"; token: string }
  // The recorded voice channels changed; the finished recordings (staff,
  // asked for); a token for one download (see lib/recording).
  | { type: "recording"; channels: number[] }
  | { type: "recordings"; recordings: FinishedRecording[] }
  | { type: "recording-token"; token: string }
  // The listen-only voice channels changed.
  | { type: "listen-only"; channels: ListenOnlyChannel[] }
  // The private channels we see changed (or their members).
  | { type: "private"; channels: PrivateChannel[] }
  // A private channel's invite links (for staff), a new link's token
  // (only to its maker, once), and the channel a redeemed link opens
  // (channel: a text channel; else the voice channel).
  | { type: "invites"; voice: number; invites: Invite[] }
  | { type: "invite-created"; voice: number; token: string }
  | { type: "invite-joined"; voice: number; channel?: number }
  // The running time-outs changed.
  | { type: "timeouts"; timeouts: Timeout[] }
  // Admins changed whether guests may only read.
  | { type: "guests-read-only"; on: boolean }
  | { type: "captcha"; level: CaptchaLevel }
  // Every connection (admins, asked for: see lib/connections).
  | { type: "connections"; connections: Connection[] }
  // The message rules (admins, asked for or saved: see lib/messageRules).
  | { type: "message-rules"; rules: MessageRules }
  // The channel that holds a voice channel's chat.
  | { type: "voice-chat"; voice: number; channel: number }
  // The text channels changed (an admin managed them).
  | { type: "channels"; channels: TextChannel[] }
  | { type: "history"; channel: number; messages: TextMessage[]; more: boolean }
  | { type: "message"; message: TextMessage }
  | { type: "message-edited"; message: TextMessage }
  // A message was pinned or unpinned (its pinned field says which).
  | { type: "message-pinned"; message: TextMessage }
  // A channel's pinned messages, the newest pin first.
  | { type: "pins"; channel: number; messages: TextMessage[] }
  // Messages of a channel were deleted (the server sends one as
  // "message-deleted").
  | { type: "messages-deleted"; channel: number; ids: number[] }
  | { type: "search-results"; text: string; messages: TextMessage[] }
  // about "manage": the answer to a channel or role change; slow: a post
  // refused by slow mode, and when the wait ends.
  // returned: a refused send's text, for the composer of its chat.
  | { type: "error"; message: string; about?: "manage"; slow?: { channel: number; until: number }; returned?: { channel: number; text: string } }
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
  uploadMax?: number; // the server's size limit per upload
  // Voice channel id -> the channel that holds its chat.
  voiceChannels: Readonly<Record<number, number>>;
  messages: Readonly<Record<number, TextMessage[]>>; // per channel, by id
  loaded: Readonly<Record<number, boolean>>;
  more: Readonly<Record<number, boolean>>;
  lastError: string | null;
  roles: Roles;
  // Listen-only voice channels: their speakers.
  listenOnly: Readonly<Record<number, Speaker[]>>;
  // Private channels we see, by Mumble channel (a voice channel, or a
  // text channel's mirror): their members (for admins and moderators;
  // others get none).
  private: Readonly<Record<number, Speaker[]>>;
  // Invite links of private channels (staff), once asked for.
  invites: Readonly<Record<number, Invite[]>>;
  timeouts: readonly Timeout[];
  guestsReadOnly: boolean;
  // Slow mode: when the server said our wait ends, by channel.
  slowUntil: Readonly<Record<number, number>>;
  // The text of the last refused send (n counts them, see ChatPane).
  returned?: { channel: number; text: string; n: number };
  captcha: CaptchaLevel;
  // Every connection, once asked for (admins).
  connections: Connection[] | null;
  // The message rules, once asked for (admins).
  messageRules: MessageRules | null;
  // Direct chats: chat -> the other user.
  directs: Readonly<Record<number, Peer>>;
  // Live screen shares, and their thumbnails by streamer session.
  streams: readonly LiveStream[];
  streamPreviews: Readonly<Record<number, string>>;
  // Recorded voice channels (null: the server does not record), and the
  // finished recordings once asked for (staff).
  recording: readonly number[] | null;
  recordings: FinishedRecording[] | null;
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
  // Pinned messages per channel, once asked for (the newest pin first).
  pins: Readonly<Record<number, TextMessage[]>>;
  // Counts resync events, so views can reload.
  resyncs: number;
  // Statuses of web clients by session (plain online: none), and ours as
  // chosen.
  statuses: Readonly<Record<number, PresenceView>>;
  ownStatus: PresenceView;
  // Custom server emoji (image URLs relative to the server).
  emoji: readonly CustomEmoji[];
  // Registered users, and the bios of offline ones once asked for (by
  // user id).
  members: Members;
  bios: Readonly<Record<number, string>>;
  // Members online, whether or not we see them (see lib/members).
  online: Online;
};

export const initialTextState: TextState = {
  available: false,
  channels: [],
  voiceChats: false,
  roles: { admins: [], moderators: [] },
  listenOnly: {},
  private: {},
  invites: {},
  timeouts: [],
  guestsReadOnly: false,
  slowUntil: {},
  captcha: "off",
  connections: null,
  messageRules: null,
  directs: {},
  streams: [],
  streamPreviews: {},
  recording: null,
  recordings: null,
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
  pins: {},
  resyncs: 0,
  statuses: {},
  ownStatus: { status: "online" },
  emoji: [],
  members: {},
  online: {},
  bios: {},
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
const parseSessionStatus = (v: unknown): SessionStatus | null => {
  const p = parsePresence(v);
  return p && isNum((v as SessionStatus).session) ? { ...p, session: (v as SessionStatus).session } : null;
};
const statusMap = (list: readonly SessionStatus[]) =>
  Object.fromEntries(list.map(({ session, ...v }) => [session, v])) as Record<number, PresenceView>;
const isPlain = (v: PresenceView) => v.status === "online" && !v.emoji && !v.text;
const isObj = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object";
const isSpeaker = (v: unknown): v is Speaker => isObj(v) && isNum(v.id) && isStr(v.name);
const isListenOnly = (v: unknown): v is ListenOnlyChannel[] =>
  Array.isArray(v) && v.every((c) => isObj(c) && isNum(c.channel) && Array.isArray(c.speakers) && c.speakers.every(isSpeaker));
const listenOnlyMap = (cs: ListenOnlyChannel[]) => Object.fromEntries(cs.map((c) => [c.channel, c.speakers]));
const isPrivateList = (v: unknown): v is PrivateChannel[] =>
  Array.isArray(v) && v.every((c) => isObj(c) && isNum(c.channel) && (c.members === undefined || (Array.isArray(c.members) && c.members.every(isSpeaker))));
const isInvite = (v: unknown): v is Invite =>
  isObj(v) &&
  isNum(v.id) &&
  isStr(v.creator) &&
  isNum(v.created) &&
  isNum(v.uses) &&
  (v.expires === undefined || isNum(v.expires)) &&
  (v.maxUses === undefined || isNum(v.maxUses));
const privateMap = (cs: PrivateChannel[]) => Object.fromEntries(cs.map((c) => [c.channel, c.members ?? []]));
const isTimeouts = (v: unknown): v is Timeout[] =>
  Array.isArray(v) && v.every((t) => isObj(t) && isNum(t.until) && (isNum(t.user) !== isNum(t.session)));
// Direct chats follow their peer's renames (a removed peer keeps its name).
const renamePeer = (directs: TextState["directs"], id: number, name: string) =>
  Object.fromEntries(Object.entries(directs).map(([ch, p]) => [ch, p.id === id ? { id, name } : p]));
const isStreamPreview = (v: unknown): v is string => isStr(v) && v.startsWith("data:image/jpeg;base64,");
const isStreams = (v: unknown): v is LiveStream[] =>
  Array.isArray(v) && v.every((s) => isObj(s) && isNum(s.session) && isNum(s.channel) && (s.preview === undefined || isStreamPreview(s.preview)));
// Thumbnails of the streams that are still live.
const keepPreviews = (previews: TextState["streamPreviews"], streams: readonly LiveStream[]) =>
  Object.fromEntries(streams.flatMap((s) => (s.preview ?? previews[s.session] ? [[s.session, s.preview ?? previews[s.session]!]] : [])));
const isDirect = (v: unknown): v is DirectChat =>
  isObj(v) && isNum(v.channel) && isObj(v.peer) && isNum(v.peer.id) && isStr(v.peer.name);

const isChannel = (v: unknown): v is TextChannel =>
  isObj(v) &&
  isNum(v.id) &&
  isStr(v.name) &&
  (v.mumble === undefined || isNum(v.mumble)) &&
  (v.topic === undefined || typeof v.topic === "string") &&
  (v.slowMode === undefined || isNum(v.slowMode));
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
  Array.isArray(v) &&
  v.every(
    (r) =>
      isObj(r) &&
      isStr(r.emoji) &&
      Array.isArray(r.users) &&
      r.users.every(isStr) &&
      Array.isArray(r.ids) &&
      r.ids.every(isNum) &&
      r.ids.length === r.users.length,
  );

const isMessage = (v: unknown): v is TextMessage =>
  isObj(v) &&
  isNum(v.id) &&
  isNum(v.channel) &&
  isStr(v.author) &&
  isStr(v.text) &&
  isNum(v.at) &&
  (v.verified === undefined || typeof v.verified === "boolean") &&
  (v.authorId === undefined || isNum(v.authorId)) &&
  (v.edited === undefined || isNum(v.edited)) &&
  (v.pinned === undefined || isNum(v.pinned)) &&
  (v.reply === undefined || (isObj(v.reply) && isNum(v.reply.id) && isStr(v.reply.author) && isStr(v.reply.text))) &&
  (v.upload === undefined || isAttachment(v.upload)) &&
  (v.reactions === undefined || isReactions(v.reactions)) &&
  (v.mentionAll === undefined || isMentionAll(v.mentionAll)) &&
  (v.noPing === undefined || typeof v.noPing === "boolean");

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
      const statuses = Array.isArray(v.statuses) ? v.statuses.flatMap((s) => parseSessionStatus(s) ?? []) : undefined;
      const ownStatus = parsePresence(v.ownStatus);
      const emoji = parseCustomEmoji(v.emoji);
      return {
        type: "ready",
        channels: v.channels,
        ...(v.voiceChats === true ? { voiceChats: true } : {}),
        ...(isNum(v.uploadMax) && v.uploadMax > 0 ? { uploadMax: v.uploadMax } : {}),
        ...(roles ? { roles } : {}),
        ...(isListenOnly(v.listenOnly) ? { listenOnly: v.listenOnly } : {}),
        ...(isPrivateList(v.private) ? { private: v.private } : {}),
        ...(isTimeouts(v.timeouts) ? { timeouts: v.timeouts } : {}),
        ...(v.guestsReadOnly === true ? { guestsReadOnly: true } : {}),
        ...(isCaptchaLevel(v.captcha) ? { captcha: v.captcha } : {}),
        ...(Array.isArray(v.directs) ? { directs: v.directs.filter(isDirect) } : {}),
        ...(isStreams(v.streams) ? { streams: v.streams } : {}),
        ...(Array.isArray(v.recording) && v.recording.every(isNum) ? { recording: v.recording } : {}),
        ...(reads ? { reads: Object.fromEntries(Object.entries(reads).map(([k, n]) => [Number(k), n])) } : {}),
        ...(summary ? { summary } : {}),
        ...(statuses ? { statuses } : {}),
        ...(ownStatus ? { ownStatus } : {}),
        ...(emoji ? { emoji } : {}),
        ...(Array.isArray(v.members) && v.members.every(isSpeaker) ? { members: v.members } : {}),
        ...(isObj(v.online) && Object.values(v.online).every(isNum) ? { online: v.online as Online } : {}),
      };
    }
    case "member":
      if (!isNum(v.id)) return null;
      return isStr(v.name) ? { type: "member", id: v.id, name: v.name } : v.gone === true ? { type: "member", id: v.id } : null;
    case "online":
      return isNum(v.id) ? { type: "online", id: v.id, ...(isNum(v.session) ? { session: v.session } : {}) } : null;
    case "bio":
      return isNum(v.user) && isStr(v.text) ? { type: "bio", user: v.user, text: v.text } : null;
    case "emoji": {
      const emoji = parseCustomEmoji(v.emoji);
      return emoji && { type: "emoji", emoji };
    }
    case "roles": {
      const roles = parseRoles(v);
      return roles && { type: "roles", ...roles };
    }
    case "listen-only":
      return isListenOnly(v.channels) ? { type: "listen-only", channels: v.channels } : null;
    case "private":
      return isPrivateList(v.channels) ? { type: "private", channels: v.channels } : null;
    case "invites":
      return isNum(v.voice) && Array.isArray(v.invites) && v.invites.every(isInvite) ? { type: "invites", voice: v.voice, invites: v.invites } : null;
    case "invite-created":
      return isNum(v.voice) && isStr(v.token) ? { type: "invite-created", voice: v.voice, token: v.token } : null;
    case "invite-joined":
      if (!isNum(v.voice) || (v.channel !== undefined && !isNum(v.channel))) return null;
      return { type: "invite-joined", voice: v.voice, ...(v.channel !== undefined && { channel: v.channel }) };
    case "status": {
      const s = parseSessionStatus(v);
      return s && { type: "status", ...s };
    }
    case "own-status": {
      const p = parsePresence(v);
      return p && { type: "own-status", ...p };
    }
    case "guests-read-only":
      return typeof v.on === "boolean" ? { type: "guests-read-only", on: v.on } : null;
    case "captcha":
      return isCaptchaLevel(v.level) ? { type: "captcha", level: v.level } : null;
    case "connections": {
      const connections = parseConnections(v.connections);
      return connections && { type: "connections", connections };
    }
    case "message-rules": {
      const rules = parseMessageRules(v.rules);
      return rules && { type: "message-rules", rules };
    }
    case "timeouts":
      return isTimeouts(v.timeouts) ? { type: "timeouts", timeouts: v.timeouts } : null;
    case "streams":
      return isStreams(v.streams) ? { type: "streams", streams: v.streams } : null;
    case "stream-token":
      return isStr(v.token) ? { type: "stream-token", token: v.token } : null;
    case "recording":
      return Array.isArray(v.channels) && v.channels.every(isNum) ? { type: "recording", channels: v.channels } : null;
    case "recordings": {
      const recordings = parseRecordings(v.recordings);
      return recordings && { type: "recordings", recordings };
    }
    case "recording-token":
      return isStr(v.token) ? { type: "recording-token", token: v.token } : null;
    case "stream-preview": // "": the preview was turned off
      return isNum(v.session) && (v.image === "" || isStreamPreview(v.image)) ? { type: "stream-preview", session: v.session, image: v.image as string } : null;
    case "dm":
      return isDirect(v) ? { type: "dm", channel: v.channel, peer: { id: v.peer.id, name: v.peer.name } } : null;
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
    case "message-pinned":
      return isMessage(v.message) ? { type: v.type, message: v.message } : null;
    case "pins":
      return isNum(v.channel) && Array.isArray(v.messages) && v.messages.every(isMessage)
        ? { type: "pins", channel: v.channel, messages: v.messages }
        : null;
    case "message-deleted":
      return isNum(v.channel) && isNum(v.id) ? { type: "messages-deleted", channel: v.channel, ids: [v.id] } : null;
    case "messages-deleted":
      return isNum(v.channel) && Array.isArray(v.ids) && v.ids.every(isNum) ? { type: "messages-deleted", channel: v.channel, ids: v.ids } : null;
    case "search-results":
      return isStr(v.text) && Array.isArray(v.messages) && v.messages.every(isMessage)
        ? { type: "search-results", text: v.text, messages: v.messages }
        : null;
    case "error":
      if (!isStr(v.message)) return null;
      const returned = isNum(v.channel) && isStr(v.text) ? { returned: { channel: v.channel, text: v.text } } : {};
      if (v.about === "slow-mode" && isNum(v.channel) && isNum(v.until))
        return { type: "error", message: v.message, slow: { channel: v.channel, until: v.until }, ...returned };
      if (v.about === "refused") return { type: "error", message: v.message, ...returned };
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

// A channel's pin list after f, if it is loaded.
const patchPins = (pins: TextState["pins"], channel: number, f: (ms: TextMessage[]) => TextMessage[]) =>
  pins[channel] ? { ...pins, [channel]: f(pins[channel]) } : pins;

// A pin list after a message was pinned or unpinned: the newest pin first.
const withPin = (list: readonly TextMessage[], m: TextMessage) =>
  [...list.filter((x) => x.id !== m.id), ...(m.pinned ? [m] : [])].sort((x, y) => (y.pinned ?? 0) - (x.pinned ?? 0));

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
        uploadMax: event.uploadMax,
        roles: event.roles ?? initialTextState.roles,
        emoji: event.emoji ?? [],
        members: Object.fromEntries((event.members ?? []).map((p) => [p.id, p.name])),
        online: event.online ?? {},
        bios: {},
        listenOnly: listenOnlyMap(event.listenOnly ?? []),
        private: privateMap(event.private ?? []),
        timeouts: event.timeouts ?? [],
        guestsReadOnly: event.guestsReadOnly === true,
        captcha: event.captcha ?? "off",
        directs: { ...state.directs, ...Object.fromEntries((event.directs ?? []).map((d) => [d.channel, d.peer])) },
        streams: event.streams ?? [],
        streamPreviews: keepPreviews({}, event.streams ?? []),
        recording: event.recording ?? null,
        reads: event.reads ?? {},
        statuses: statusMap(event.statuses ?? []),
        ownStatus: event.ownStatus ?? initialTextState.ownStatus,
        summary: Object.fromEntries((event.summary ?? []).map((e) => [e.channel, { latest: e.latest, mentions: e.mentions }])),
        voiceChannels: {
          ...state.voiceChannels,
          ...Object.fromEntries((event.summary ?? []).flatMap((e) => (e.voice === undefined ? [] : [[e.voice, e.channel]]))),
        },
      };
    case "roles":
      return { ...state, roles: { admins: event.admins, moderators: event.moderators } };
    case "emoji":
      return { ...state, emoji: event.emoji };
    case "member": {
      const { [event.id]: _, ...rest } = state.members;
      if (event.name === undefined) return { ...state, members: rest };
      return { ...state, members: { ...rest, [event.id]: event.name }, directs: renamePeer(state.directs, event.id, event.name) };
    }
    case "online": {
      const { [event.id]: _, ...rest } = state.online;
      return { ...state, online: event.session === undefined ? rest : { ...rest, [event.id]: event.session } };
    }
    case "bio":
      return { ...state, bios: { ...state.bios, [event.user]: event.text } };
    case "listen-only":
      return { ...state, listenOnly: listenOnlyMap(event.channels) };
    case "private":
      return { ...state, private: privateMap(event.channels) };
    case "invites":
      return { ...state, invites: { ...state.invites, [event.voice]: event.invites } };
    case "invite-created": // the client shows it once (lib/invites)
    case "invite-joined":
      return state;
    case "status": {
      const { type: _, session, ...v } = event;
      const { [session]: __, ...rest } = state.statuses;
      return { ...state, statuses: isPlain(v) ? rest : { ...rest, [session]: v } };
    }
    case "own-status": {
      const { type: _, ...v } = event;
      return { ...state, ownStatus: v };
    }
    case "timeouts":
      return { ...state, timeouts: event.timeouts };
    case "guests-read-only":
      return { ...state, guestsReadOnly: event.on };
    case "captcha":
      return { ...state, captcha: event.level };
    case "connections":
      return { ...state, connections: event.connections };
    case "message-rules":
      return { ...state, messageRules: event.rules };
    case "streams":
      return { ...state, streams: event.streams, streamPreviews: keepPreviews(state.streamPreviews, event.streams) };
    case "stream-preview": {
      const { [event.session]: _, ...rest } = state.streamPreviews;
      return { ...state, streamPreviews: event.image ? { ...rest, [event.session]: event.image } : rest };
    }
    case "stream-token": // answered by the connection, not state
    case "recording-token":
      return state;
    case "recording":
      return { ...state, recording: event.channels };
    case "recordings":
      return { ...state, recordings: event.recordings };
    case "dm":
      return state.directs[event.channel]?.name === event.peer.name
        ? state
        : { ...state, directs: { ...state.directs, [event.channel]: event.peer } };
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
      const replace = (ms: TextMessage[]) => ms.map((m) => (m.id === event.message.id ? event.message : m));
      const search = event.type === "message-edited" ? patchSearch(state.search, replace) : state.search;
      const pins = event.type === "message-edited" ? patchPins(state.pins, ch, replace) : state.pins;
      // An edit of a message not loaded here stays unloaded.
      if (event.type === "message-edited" && !state.messages[ch]?.some((m) => m.id === event.message.id)) return { ...state, search, pins };
      // An edit may change the link: its preview is asked again.
      const { [event.message.id]: _, ...previews } = state.previews;
      return { ...state, search, pins, previews, messages: { ...state.messages, [ch]: merge(state.messages[ch], [event.message]) } };
    }
    case "message-pinned": {
      const m = event.message;
      const msgs = state.messages[m.channel];
      const list = state.pins[m.channel];
      return {
        ...state,
        ...(msgs?.some((x) => x.id === m.id) && { messages: { ...state.messages, [m.channel]: msgs.map((x) => (x.id === m.id ? m : x)) } }),
        ...(list && { pins: { ...state.pins, [m.channel]: withPin(list, m) } }),
      };
    }
    case "pins":
      return { ...state, pins: { ...state.pins, [event.channel]: event.messages } };
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
    case "messages-deleted": {
      const gone = new Set(event.ids);
      const drop = (ms: TextMessage[]) => ms.filter((m) => !gone.has(m.id));
      const search = patchSearch(state.search, drop);
      const pins = patchPins(state.pins, event.channel, drop);
      const msgs = state.messages[event.channel];
      if (!msgs) return { ...state, search, pins };
      return { ...state, search, pins, messages: { ...state.messages, [event.channel]: drop(msgs) } };
    }
    case "error":
      return event.about === "manage"
        ? { ...state, notice: { text: event.message, n: (state.notice?.n ?? 0) + 1 } }
        : {
            ...state,
            lastError: event.message,
            ...(event.slow && { slowUntil: { ...state.slowUntil, [event.slow.channel]: event.slow.until } }),
            ...(event.returned && { returned: { ...event.returned, n: (state.returned?.n ?? 0) + 1 } }),
          };
    case "upload-token": // answered by the connection, not state
    case "typing": // short-lived: the client keeps it (lib/typing)
      return state;
    case "resync":
      return { ...state, messages: {}, loaded: {}, more: {}, pins: {}, resyncs: state.resyncs + 1 };
  }
};

// A user left: its status goes (Murmur reuses session numbers).
export const forgetStatus = (state: TextState, session: number): TextState => {
  if (!state.statuses[session]) return state;
  const { [session]: _, ...statuses } = state.statuses;
  return { ...state, statuses };
};

// A request for older messages (before: the oldest id shown) is answered
// once older messages are here or the channel has no more; a resync that
// drops the history ends it too.
export const pageAnswered = (state: TextState, page: { channel: number; before: number }) => {
  const oldest = state.messages[page.channel]?.[0]?.id;
  return oldest === undefined || oldest < page.before || state.more[page.channel] === false;
};

export const historyRequest = (channel: number, before?: number) =>
  JSON.stringify(before === undefined ? { type: "history", channel } : { type: "history", channel, before });

export const sendRequest = (channel: number, text: string, upload?: string, reply?: number, noPing?: boolean) =>
  JSON.stringify({ type: "send", channel, text, ...(upload ? { upload } : {}), ...(reply ? { reply } : {}), ...(reply && noPing ? { noPing } : {}) });

export const uploadTokenRequest = () => JSON.stringify({ type: "upload-token" });
// Custom emoji (admins and moderators; the server checks): a token to
// upload one, and deleting one.
export const emojiTokenRequest = (name: string) => JSON.stringify({ type: "emoji-token", name });
export const deleteEmojiRequest = (name: string) => JSON.stringify({ type: "delete-emoji", name });

export const voiceChatRequest = (voice: number) => JSON.stringify({ type: "voice-chat", voice });

// Text channel management (admins; the server checks).
export type ManageAction =
  | { create: string; private?: boolean }
  | { createPrivateVoice: string }
  // A Mumble channel (see TextState.private) private with these members,
  // or public.
  | { setPrivate: number; on: boolean; users: number[] }
  // Invite links of a private Mumble channel (see lib/invites): list,
  // make (seconds 0: for ever; uses 0: unlimited), delete, redeem.
  | { invites: number }
  | { createInvite: number; seconds: number; uses: number }
  | { deleteInvite: number }
  | { redeemInvite: string }
  | { rename: number; name: string }
  | { remove: number }
  | { order: number[] }
  | { moderator: number; on: boolean }
  | { admin: number; on: boolean }
  | { listenOnly: number; on: boolean; users: number[] }
  | { topic: number; text: string }
  | { slowMode: number; seconds: number }
  // Moderation, by voice server session: a ban for some seconds (0: for
  // ever), a time-out (0: end it).
  | { ban: number; reason: string; seconds: number }
  | { timeout: number; seconds: number }
  // A user's messages of the last seconds (staff): see UserTarget.
  | { deleteMessages: UserTarget; seconds: number }
  | { guestsReadOnly: boolean }
  | { captcha: CaptchaLevel }
  // Every connection (admins).
  | { connections: true }
  // The message rules (admins): ask for them, or save new ones.
  | { messageRules: true }
  | { setMessageRules: MessageRules }
  // Recording a voice channel (staff): start or stop; the finished ones.
  | { record: number; on: boolean }
  | { recordings: true };

// A user for the Kallisti server: by voice server session if online, else
// a registered user by id.
export type UserTarget = { session: number } | { user: number };

const manageBody = (a: ManageAction) => {
  if ("create" in a) return { type: "create-channel", name: a.create, ...(a.private ? { private: true } : {}) };
  if ("createPrivateVoice" in a) return { type: "create-private-voice", name: a.createPrivateVoice };
  if ("setPrivate" in a) return { type: "set-private", voice: a.setPrivate, on: a.on, users: a.users };
  if ("invites" in a) return { type: "invites", voice: a.invites };
  if ("createInvite" in a) return { type: "create-invite", voice: a.createInvite, seconds: a.seconds, uses: a.uses };
  if ("deleteInvite" in a) return { type: "delete-invite", invite: a.deleteInvite };
  if ("redeemInvite" in a) return { type: "redeem-invite", token: a.redeemInvite };
  if ("rename" in a) return { type: "rename-channel", channel: a.rename, name: a.name };
  if ("remove" in a) return { type: "delete-channel", channel: a.remove };
  if ("moderator" in a) return { type: "set-moderator", user: a.moderator, moderator: a.on };
  if ("admin" in a) return { type: "set-admin", user: a.admin, admin: a.on };
  if ("listenOnly" in a) return { type: "set-listen-only", voice: a.listenOnly, on: a.on, users: a.users };
  if ("topic" in a) return { type: "set-topic", channel: a.topic, text: a.text };
  if ("slowMode" in a) return { type: "set-slow-mode", channel: a.slowMode, seconds: a.seconds };
  if ("ban" in a) return { type: "ban", session: a.ban, text: a.reason, seconds: a.seconds };
  if ("timeout" in a) return { type: "time-out", session: a.timeout, seconds: a.seconds };
  if ("deleteMessages" in a) return { type: "delete-messages", ...a.deleteMessages, seconds: a.seconds };
  if ("guestsReadOnly" in a) return { type: "set-guests-read-only", on: a.guestsReadOnly };
  if ("captcha" in a) return { type: "set-captcha", level: a.captcha };
  if ("connections" in a) return { type: "connections" };
  if ("messageRules" in a) return { type: "message-rules" };
  if ("setMessageRules" in a) return { type: "set-message-rules", rules: a.setMessageRules };
  if ("record" in a) return { type: a.on ? "start-recording" : "stop-recording", voice: a.record };
  if ("recordings" in a) return { type: "recordings" };
  return { type: "order-channels", channels: a.order };
};

export const manageRequest = (a: ManageAction) => JSON.stringify(manageBody(a));

export const searchRequest = (q: SearchQuery) => JSON.stringify({ type: "search", ...q });
export const directRequest = (user: number) => JSON.stringify({ type: "dm", user });

// The direct chat with a user, if known.
export const directChatWith = (directs: TextState["directs"], user: number): number | null => {
  const found = Object.entries(directs).find(([, p]) => p.id === user);
  return found ? Number(found[0]) : null;
};
export const pinRequest = (message: number, on: boolean) => JSON.stringify({ type: "pin", message, on });
export const pinsRequest = (channel: number) => JSON.stringify({ type: "pins", channel });
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

const chatCache = new WeakMap<TextMessage, { chat: DisplayMessage; channels?: ReadonlyMap<string, number> }>();

// Text channel messages are plain text: escape, then Markdown. Attachment
// URLs are relative to the server (`base`); channels: text channel ids by
// name, for "#name" links (a new map makes the line again).
export const toChatMessage = (m: TextMessage, base = "", channels?: ReadonlyMap<string, number>): DisplayMessage => {
  const cached = chatCache.get(m);
  let chat = cached?.channels === channels ? cached?.chat : undefined;
  if (!chat) {
    const url = m.upload && fileUrl(m.upload.url, base);
    chat = {
      actor: null,
      actorName: m.author,
      channelIds: [],
      treeIds: [],
      sessions: [],
      text: markdown(escapeHtml(m.text), channels),
      at: m.at,
      peer: null,
      verified: m.verified === true,
      ...(m.authorId !== undefined && { authorId: m.authorId }),
      source: { id: m.id, text: m.text },
      ...(m.edited !== undefined && { edited: m.edited }),
      ...(m.pinned !== undefined && { pinned: m.pinned }),
      ...(m.reply && { reply: m.reply }),
      ...(m.reactions?.length && { reactions: m.reactions }),
      ...(m.mentionAll && { mentionAll: m.mentionAll }),
      ...(m.noPing && { noPing: true }),
      ...(m.upload && url && { upload: spoilerUpload({ ...m.upload, url }) }),
    };
    chatCache.set(m, { chat, channels });
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
