import { create, isFieldSet, type DescField, type Message } from "@bufbuild/protobuf";
import { textMessageInit, type TextTarget } from "./actions";
import * as pb from "./gen/Mumble_pb";
import type { MumbleMessage } from "./messages";

export type Channel = {
  id: number;
  parent: number | null;
  name: string;
  description: string;
  // Set instead of `description` when the text is long (128 bytes or more).
  // Fetch the text with a RequestBlob (see missingDescriptions).
  descriptionHash: string | null;
  position: number;
  temporary: boolean;
};

export type User = {
  session: number;
  name: string;
  channelId: number;
  // Set for users registered on the server.
  userId: number | null;
  mute: boolean;
  deaf: boolean;
  suppress: boolean;
  selfMute: boolean;
  selfDeaf: boolean;
  prioritySpeaker: boolean;
  recording: boolean;
  // The avatar picture, as sent (null: none, or not loaded yet). Long ones
  // come as a hash first: fetch them with a RequestBlob (missingTextures).
  texture: Uint8Array | null;
  textureHash: string | null;
  // The comment (the bio, HTML). Long ones come as a hash first, with the
  // text "" until fetched (see commentsToFetch).
  comment: string;
  commentHash: string | null;
};

export type ChatMessage = {
  actor: number | null;
  actorName: string | null;
  channelIds: number[];
  treeIds: number[];
  sessions: number[];
  text: string;
  at: number;
  // Private messages: the other person's name. Conversations are keyed by
  // name, because Murmur reuses session numbers of users who left.
  peer: string | null;
};

export type Version = { major: number; minor: number; patch: number };

export type ServerState = {
  serverVersion: Version | null;
  status: "connecting" | "synced" | "rejected" | "kicked";
  session: number | null;
  welcomeText: string;
  error: string | null;
  rejectType: pb.Reject_RejectType | null;
  channels: Readonly<Record<number, Channel>>;
  users: Readonly<Record<number, User>>;
  messages: readonly ChatMessage[];
  // Private messages received per sender name. Not capped like `messages`.
  privateCounts: Readonly<Record<string, number>>;
  // The last thing the server refused (PermissionDenied), for a short notice.
  notice: { text: string; at: number } | null;
  // What we may do in the root channel (ServerSync), null before it.
  rootPermissions: number | null;
  // What we may do in other channels, from PermissionQuery answers.
  channelPermissions: Readonly<Record<number, number>>;
  // Admin views (null: not asked yet): the ban list and the registered
  // users, as the server last sent them.
  bans: readonly Ban[] | null;
  registered: readonly Registered[] | null;
};

// A ban as Mumble sends it: a 16-byte address (IPv4 as ::ffff:a.b.c.d)
// with a prefix length over 128 bits; duration 0: for ever.
export type Ban = {
  address: Uint8Array;
  mask: number;
  name: string;
  hash: string;
  reason: string;
  start: string;
  duration: number;
};
export type Registered = { userId: number; name: string };

export const MAX_MESSAGES = 1000;

export const initialState: ServerState = {
  rootPermissions: null,
  channelPermissions: {},
  bans: null,
  registered: null,
  serverVersion: null,
  status: "connecting",
  session: null,
  welcomeText: "",
  error: null,
  rejectType: null,
  channels: {},
  users: {},
  messages: [],
  privateCounts: {},
  notice: null,
};

// Mumble sends partial updates: only fields that are set on the wire apply.
const pickSet = <T extends Message, K extends keyof T & string>(
  msg: T,
  schema: { field: Record<string, DescField> },
  keys: readonly K[],
): Partial<Pick<T, K>> =>
  Object.fromEntries(
    keys.filter((k) => isFieldSet(msg, schema.field[k]!)).map((k) => [k, msg[k]]),
  ) as Partial<Pick<T, K>>;

export const emptyChannel = (id: number): Channel => ({
  id,
  parent: null,
  name: "",
  description: "",
  descriptionHash: null,
  position: 0,
  temporary: false,
});

export const emptyUser = (session: number): User => ({
  session,
  name: "",
  channelId: 0,
  userId: null,
  mute: false,
  deaf: false,
  suppress: false,
  selfMute: false,
  selfDeaf: false,
  prioritySpeaker: false,
  recording: false,
  texture: null,
  textureHash: null,
  comment: "",
  commentHash: null,
});

const parseVersion = (msg: pb.Version): Version =>
  isFieldSet(msg, pb.VersionSchema.field.versionV2)
    ? {
        major: Number(msg.versionV2 >> 48n),
        minor: Number((msg.versionV2 >> 32n) & 0xffffn),
        patch: Number((msg.versionV2 >> 16n) & 0xffffn),
      }
    : {
        major: msg.versionV1 >>> 16,
        minor: (msg.versionV1 >> 8) & 0xff,
        patch: msg.versionV1 & 0xff,
      };

const without = <V>(record: Readonly<Record<number, V>>, key: number) => {
  const { [key]: _, ...rest } = record;
  return rest;
};

// The userId Murmur sends when a registration is removed.
const UNREGISTERED = 0xffffffff;

// As Murmur: from this size, data comes as a hash first.
const BLOB_HASH_MIN = 128;

const hex = (b: Uint8Array) => [...b].map((x) => x.toString(16).padStart(2, "0")).join("");

// Text that is set and short: it always comes inline.
const isShortText = (msg: Message, field: DescField, text: string) =>
  isFieldSet(msg, field) && new TextEncoder().encode(text).length < BLOB_HASH_MIN;

const applyChannelState = (state: ServerState, msg: pb.ChannelState): ServerState => {
  const prev = state.channels[msg.channelId] ?? emptyChannel(msg.channelId);
  const hash = isFieldSet(msg, pb.ChannelStateSchema.field.descriptionHash)
    ? hex(msg.descriptionHash)
    : prev.descriptionHash;
  const next: Channel = {
    ...prev,
    // A new hash: the fetched text is out of date.
    ...(hash !== prev.descriptionHash && { description: "" }),
    ...pickSet(msg, pb.ChannelStateSchema, [
      "parent",
      "name",
      "description",
      "position",
      "temporary",
    ]),
    // Short texts always come inline, so any earlier hash is out of date.
    descriptionHash: isShortText(msg, pb.ChannelStateSchema.field.description, msg.description) ? null : hash,
  };
  return { ...state, channels: { ...state.channels, [msg.channelId]: next } };
};

const applyUserState = (state: ServerState, msg: pb.UserState): ServerState => {
  const prev = state.users[msg.session] ?? emptyUser(msg.session);
  const next: User = {
    ...prev,
    ...pickSet(msg, pb.UserStateSchema, [
      "name",
      "channelId",
      "mute",
      "deaf",
      "suppress",
      "selfMute",
      "selfDeaf",
      "prioritySpeaker",
      "recording",
      "userId",
    ]),
  };
  // Comments work as channel descriptions (see applyChannelState).
  const commentHash = isFieldSet(msg, pb.UserStateSchema.field.commentHash) ? hex(msg.commentHash) : prev.commentHash;
  if (commentHash !== prev.commentHash) next.comment = "";
  if (isFieldSet(msg, pb.UserStateSchema.field.comment)) next.comment = msg.comment;
  next.commentHash = isShortText(msg, pb.UserStateSchema.field.comment, msg.comment) ? null : commentHash;
  if (next.userId === UNREGISTERED) next.userId = null;
  if (isFieldSet(msg, pb.UserStateSchema.field.textureHash)) {
    const hash = hex(msg.textureHash);
    // The same picture again keeps its data.
    if (hash !== prev.textureHash) next.texture = null;
    next.textureHash = hash;
  }
  if (isFieldSet(msg, pb.UserStateSchema.field.texture)) {
    // Empty: the picture was removed. Short ones always come inline, so
    // any earlier hash is out of date; long ones answer our RequestBlob.
    next.texture = msg.texture.length ? msg.texture.slice() : null;
    if (msg.texture.length < BLOB_HASH_MIN) next.textureHash = null;
  }
  return { ...state, users: { ...state.users, [msg.session]: next } };
};

const applyTextMessage = (
  state: ServerState,
  msg: pb.TextMessage,
  now: number,
): ServerState => {
  const actor = isFieldSet(msg, pb.TextMessageSchema.field.actor) ? msg.actor : null;
  const actorName = actor === null ? null : (state.users[actor]?.name ?? null);
  const isPrivate = !msg.channelId.length && !msg.treeId.length && msg.session.length > 0;
  const incoming = actor !== state.session;
  const peer = !isPrivate ? null : incoming ? actorName : (state.users[msg.session[0]!]?.name ?? null);
  const chat: ChatMessage = {
    actor,
    actorName,
    channelIds: [...msg.channelId],
    treeIds: [...msg.treeId],
    sessions: [...msg.session],
    text: msg.message,
    at: now,
    peer,
  };
  // Long sessions: keep only the newest messages in memory.
  const messages = [...state.messages, chat];
  const privateCounts =
    isPrivate && incoming && peer !== null
      ? { ...state.privateCounts, [peer]: (state.privateCounts[peer] ?? 0) + 1 }
      : state.privateCounts;
  return {
    ...state,
    messages: messages.length > MAX_MESSAGES ? messages.slice(-MAX_MESSAGES) : messages,
    privateCounts,
  };
};

export const reduce = (
  state: ServerState,
  msg: MumbleMessage,
  now: number,
): ServerState => {
  switch (msg.type) {
    case "Version":
      return { ...state, serverVersion: parseVersion(msg.message) };
    case "ServerSync":
      return {
        ...state,
        status: "synced",
        session: msg.message.session,
        welcomeText: msg.message.welcomeText,
        // Proto2: an unset field reads as 0, which would mean "no rights".
        rootPermissions: isFieldSet(msg.message, pb.ServerSyncSchema.field.permissions)
          ? Number(msg.message.permissions)
          : null,
      };
    case "PermissionQuery": {
      if (!isFieldSet(msg.message, pb.PermissionQuerySchema.field.permissions)) return state;
      const { channelId, permissions, flush } = msg.message;
      // After ACL changes the server flushes: older answers are stale.
      const known = flush ? {} : state.channelPermissions;
      return {
        ...state,
        channelPermissions: { ...known, [channelId]: permissions },
        rootPermissions: channelId === 0 ? permissions : state.rootPermissions,
      };
    }
    case "Reject":
      return {
        ...state,
        status: "rejected",
        error: msg.message.reason,
        rejectType: msg.message.type,
      };
    case "ChannelState":
      return applyChannelState(state, msg.message);
    case "ChannelRemove":
      return { ...state, channels: without(state.channels, msg.message.channelId) };
    case "UserState":
      return applyUserState(state, msg.message);
    case "UserRemove": {
      const users = without(state.users, msg.message.session);
      return msg.message.session === state.session
        ? { ...state, users, status: "kicked", error: msg.message.reason }
        : { ...state, users };
    }
    case "TextMessage":
      return applyTextMessage(state, msg.message, now);
    case "PermissionDenied":
      return { ...state, notice: { text: msg.message.reason || "The server did not allow this.", at: now } };
    case "BanList":
      return {
        ...state,
        bans: msg.message.bans.map((b) => ({
          address: b.address.slice(),
          mask: b.mask,
          name: b.name,
          hash: b.hash,
          reason: b.reason,
          start: b.start,
          duration: b.duration,
        })),
      };
    case "UserList":
      return {
        ...state,
        registered: msg.message.users
          .map((u) => ({ userId: u.userId, name: u.name }))
          .sort((a, b) => a.userId - b.userId),
      };
    default:
      return state;
  }
};

// The server does not echo our own text messages, so add them locally.
// Takes plain text and a target, like `sendText`.
export const addOwnMessage = (
  state: ServerState,
  target: TextTarget,
  text: string,
  now: number,
): ServerState =>
  applyTextMessage(
    state,
    create(pb.TextMessageSchema, {
      ...(state.session === null ? {} : { actor: state.session }),
      ...textMessageInit(target, text),
    }),
    now,
  );
