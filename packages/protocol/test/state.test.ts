import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import * as pb from "../src/gen/Mumble_pb";
import type { MumbleMessage } from "../src/messages";
import { emptyChannel, emptyUser, initialState, reduce, type ServerState } from "../src/state";

const userState = (init: MessageInitShape<typeof pb.UserStateSchema>): MumbleMessage => ({
  type: "UserState",
  message: create(pb.UserStateSchema, init),
});
const channelState = (
  init: MessageInitShape<typeof pb.ChannelStateSchema>,
): MumbleMessage => ({ type: "ChannelState", message: create(pb.ChannelStateSchema, init) });

const run = (msgs: MumbleMessage[], state: ServerState = initialState) =>
  msgs.reduce((s, m, i) => reduce(s, m, 1000 + i), state);

describe("reduce", () => {
  it("starts in the connecting state", () => {
    expect(initialState.status).toBe("connecting");
  });

  it("marks the state as synced on ServerSync", () => {
    const s = run([
      {
        type: "ServerSync",
        message: create(pb.ServerSyncSchema, { session: 3, welcomeText: "hi" }),
      },
    ]);
    expect(s).toMatchObject({ status: "synced", session: 3, welcomeText: "hi" });
  });

  it("stores the reason on Reject", () => {
    const s = run([
      {
        type: "Reject",
        message: create(pb.RejectSchema, {
          type: pb.Reject_RejectType.UsernameInUse,
          reason: "taken",
        }),
      },
    ]);
    expect(s).toMatchObject({
      status: "rejected",
      error: "taken",
      rejectType: pb.Reject_RejectType.UsernameInUse,
    });
  });

  it("adds a channel with defaults", () => {
    const s = run([channelState({ channelId: 0, name: "Root" })]);
    expect(s.channels[0]).toEqual({ ...emptyChannel(0), name: "Root" });
    expect(emptyChannel(0)).toMatchObject({ parent: null, position: 0 });
  });

  it("merges only the fields present in a channel update", () => {
    const s = run([
      channelState({ channelId: 2, parent: 0, name: "Games", position: 5 }),
      channelState({ channelId: 2, name: "Gaming" }),
    ]);
    expect(s.channels[2]).toMatchObject({ name: "Gaming", parent: 0, position: 5 });
  });

  it("removes a channel", () => {
    const s = run([
      channelState({ channelId: 2, name: "x" }),
      { type: "ChannelRemove", message: create(pb.ChannelRemoveSchema, { channelId: 2 }) },
    ]);
    expect(s.channels[2]).toBeUndefined();
  });

  it("adds a user with defaults", () => {
    const s = run([userState({ session: 7, name: "bob", channelId: 2 })]);
    expect(s.users[7]).toEqual({ ...emptyUser(7), name: "bob", channelId: 2 });
    expect(emptyUser(7)).toMatchObject({ userId: null, channelId: 0, selfMute: false });
  });

  it("merges only the fields present in a user update", () => {
    const s = run([
      userState({ session: 7, name: "bob", channelId: 2, selfMute: true }),
      userState({ session: 7, channelId: 4 }),
      userState({ session: 7, selfMute: false }),
    ]);
    expect(s.users[7]).toMatchObject({ name: "bob", channelId: 4, selfMute: false });
  });

  it("treats explicit zero values as set", () => {
    const s = run([
      userState({ session: 7, name: "bob", channelId: 4 }),
      userState({ session: 7, channelId: 0 }),
    ]);
    expect(s.users[7]?.channelId).toBe(0);
  });

  it("moves a channel to a new parent", () => {
    const s = run([
      channelState({ channelId: 2, parent: 0, name: "x" }),
      channelState({ channelId: 2, parent: 1 }),
    ]);
    expect(s.channels[2]?.parent).toBe(1);
  });

  it("stores the user ID of registered users", () => {
    const s = run([userState({ session: 7, name: "bob", userId: 0 })]);
    expect(s.users[7]?.userId).toBe(0);
  });

  it("removes a user", () => {
    const s = run([
      userState({ session: 7, name: "bob" }),
      { type: "UserRemove", message: create(pb.UserRemoveSchema, { session: 7 }) },
    ]);
    expect(s.users[7]).toBeUndefined();
  });

  it("marks the own session as kicked when it is removed", () => {
    const s = run([
      { type: "ServerSync", message: create(pb.ServerSyncSchema, { session: 7 }) },
      {
        type: "UserRemove",
        message: create(pb.UserRemoveSchema, { session: 7, reason: "bye" }),
      },
    ]);
    expect(s).toMatchObject({ status: "kicked", error: "bye" });
  });

  it("appends text messages with the sender name at that time", () => {
    const s = run([
      userState({ session: 7, name: "bob" }),
      {
        type: "TextMessage",
        message: create(pb.TextMessageSchema, { actor: 7, channelId: [0], message: "yo" }),
      },
    ]);
    expect(s.messages).toEqual([
      {
        actor: 7,
        actorName: "bob",
        channelIds: [0],
        treeIds: [],
        sessions: [],
        text: "yo",
        at: 1001,
      },
    ]);
  });

  it("keeps tree targets of text messages", () => {
    const s = run([
      { type: "TextMessage", message: create(pb.TextMessageSchema, { treeId: [3], message: "x" }) },
    ]);
    expect(s.messages[0]?.treeIds).toEqual([3]);
  });

  it("uses null as actor for server messages", () => {
    const s = run([
      { type: "TextMessage", message: create(pb.TextMessageSchema, { message: "notice" }) },
    ]);
    expect(s.messages[0]).toMatchObject({ actor: null, actorName: null });
  });

  it("does not mutate the previous state", () => {
    const before = run([userState({ session: 7, name: "bob" })]);
    const snapshot = structuredClone(before);
    reduce(before, userState({ session: 7, name: "robert" }), 0);
    expect(before).toEqual(snapshot);
  });

  it("ignores messages it does not handle", () => {
    const s = reduce(initialState, { type: "UDPTunnel", packet: new Uint8Array() }, 0);
    expect(s).toBe(initialState);
  });
});
