import { create, isFieldSet, type DescField, type Message } from "@bufbuild/protobuf";
import { escapeHtml } from "./actions";
import * as pb from "./gen/Mumble_pb";
import type { MumbleMessage } from "./messages";

export type Channel = {
  id: number;
  parent: number | null;
  name: string;
  description: string;
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
};

export type ChatMessage = {
  actor: number | null;
  actorName: string | null;
  channelIds: number[];
  treeIds: number[];
  sessions: number[];
  text: string;
  at: number;
};

export type ServerState = {
  status: "connecting" | "synced" | "rejected" | "kicked";
  session: number | null;
  welcomeText: string;
  error: string | null;
  rejectType: pb.Reject_RejectType | null;
  channels: Readonly<Record<number, Channel>>;
  users: Readonly<Record<number, User>>;
  messages: readonly ChatMessage[];
};

export const initialState: ServerState = {
  status: "connecting",
  session: null,
  welcomeText: "",
  error: null,
  rejectType: null,
  channels: {},
  users: {},
  messages: [],
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
});

const without = <V>(record: Readonly<Record<number, V>>, key: number) => {
  const { [key]: _, ...rest } = record;
  return rest;
};

const applyChannelState = (state: ServerState, msg: pb.ChannelState): ServerState => {
  const prev = state.channels[msg.channelId] ?? emptyChannel(msg.channelId);
  const next = {
    ...prev,
    ...pickSet(msg, pb.ChannelStateSchema, [
      "parent",
      "name",
      "description",
      "position",
      "temporary",
    ]),
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
  return { ...state, users: { ...state.users, [msg.session]: next } };
};

const applyTextMessage = (
  state: ServerState,
  msg: pb.TextMessage,
  now: number,
): ServerState => {
  const actor = isFieldSet(msg, pb.TextMessageSchema.field.actor) ? msg.actor : null;
  const chat: ChatMessage = {
    actor,
    actorName: actor === null ? null : (state.users[actor]?.name ?? null),
    channelIds: [...msg.channelId],
    treeIds: [...msg.treeId],
    sessions: [...msg.session],
    text: msg.message,
    at: now,
  };
  return { ...state, messages: [...state.messages, chat] };
};

export const reduce = (
  state: ServerState,
  msg: MumbleMessage,
  now: number,
): ServerState => {
  switch (msg.type) {
    case "ServerSync":
      return {
        ...state,
        status: "synced",
        session: msg.message.session,
        welcomeText: msg.message.welcomeText,
      };
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
    default:
      return state;
  }
};

// The server does not echo our own text messages, so add them locally.
// Takes plain text, like `sendText`.
export const addOwnMessage = (
  state: ServerState,
  channelId: number,
  text: string,
  now: number,
): ServerState =>
  applyTextMessage(
    state,
    create(pb.TextMessageSchema, {
      ...(state.session === null ? {} : { actor: state.session }),
      channelId: [channelId],
      message: escapeHtml(text),
    }),
    now,
  );
