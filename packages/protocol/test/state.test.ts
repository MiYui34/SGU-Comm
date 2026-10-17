import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import * as pb from "../src/gen/Mumble_pb";
import type { MumbleMessage } from "../src/messages";
import { banAddress, missingDescriptions, privateMessagesWith } from "../src/selectors";
import {
  addOwnMessage,
  emptyChannel,
  emptyUser,
  initialState,
  MAX_MESSAGES,
  reduce,
  type ServerState,
} from "../src/state";

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

  it("stores the server version from the new format", () => {
    const s = run([
      {
        type: "Version",
        message: create(pb.VersionSchema, {
          versionV1: 0x010400,
          versionV2: (1n << 48n) | (5n << 32n) | (915n << 16n),
        }),
      },
    ]);
    expect(s.serverVersion).toEqual({ major: 1, minor: 5, patch: 915 });
  });

  it("falls back to the legacy version format", () => {
    const s = run([{ type: "Version", message: create(pb.VersionSchema, { versionV1: 0x010403 }) }]);
    expect(s.serverVersion).toEqual({ major: 1, minor: 4, patch: 3 });
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

  it("treats a removed registration as unregistered", () => {
    const s = run([
      userState({ session: 7, name: "bob", userId: 4 }),
      userState({ session: 7, userId: 0xffffffff }),
    ]);
    expect(s.users[7]?.userId).toBeNull();
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
        peer: null,
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

describe("addOwnMessage", () => {
  it("adds a sent message, because the server does not echo it", () => {
    const s = addOwnMessage(
      run([
        { type: "ServerSync", message: create(pb.ServerSyncSchema, { session: 7 }) },
        userState({ session: 7, name: "me" }),
      ]),
      { channelId: 3 },
      "hi & bye",
      5000,
    );
    expect(s.messages.at(-1)).toEqual({
      actor: 7,
      actorName: "me",
      channelIds: [3],
      treeIds: [],
      sessions: [],
      text: "hi &amp; bye",
      at: 5000,
      peer: null,
    });
  });

  it("adds a sent private message with the target session", () => {
    const s = addOwnMessage(
      run([{ type: "ServerSync", message: create(pb.ServerSyncSchema, { session: 7 }) }]),
      { session: 4 },
      "psst",
      1,
    );
    expect(s.messages.at(-1)).toMatchObject({ actor: 7, channelIds: [], sessions: [4] });
  });
});

describe("message cap", () => {
  it("keeps only the newest MAX_MESSAGES messages", () => {
    const text = (i: number): MumbleMessage => ({
      type: "TextMessage",
      message: create(pb.TextMessageSchema, { channelId: [0], message: `m${i}` }),
    });
    const s = Array.from({ length: MAX_MESSAGES + 5 }, (_, i) => text(i)).reduce(
      (acc, m, i) => reduce(acc, m, i),
      initialState,
    );
    expect(s.messages).toHaveLength(MAX_MESSAGES);
    expect(s.messages[0]?.text).toBe("m5");
    expect(s.messages.at(-1)?.text).toBe(`m${MAX_MESSAGES + 4}`);
  });
});

describe("description hashes", () => {
  it("stores the hash of a long description", () => {
    const s = run([channelState({ channelId: 2, name: "x", descriptionHash: new Uint8Array([1, 2, 3]) })]);
    expect(s.channels[2]?.descriptionHash).toBe("010203");
    expect(s.channels[2]?.description).toBe("");
  });

  it("drops the fetched text when the hash changes", () => {
    const first = [
      channelState({ channelId: 2, name: "x", descriptionHash: new Uint8Array([1]) }),
      channelState({ channelId: 2, description: "a".repeat(200) }),
    ];
    const same = run([...first, channelState({ channelId: 2, descriptionHash: new Uint8Array([1]) })]);
    expect(same.channels[2]?.description).toBe("a".repeat(200));
    const changed = run([...first, channelState({ channelId: 2, descriptionHash: new Uint8Array([2]) })]);
    expect(changed.channels[2]).toMatchObject({ description: "", descriptionHash: "02" });
    expect(missingDescriptions(changed)).toEqual([2]);
  });

  it("drops the hash when a short description comes inline", () => {
    const long = channelState({ channelId: 2, descriptionHash: new Uint8Array([1]) });
    const s = run([
      long,
      channelState({ channelId: 2, description: "a".repeat(200) }),
      channelState({ channelId: 2, description: "short" }),
    ]);
    expect(s.channels[2]?.descriptionHash).toBeNull();
    // Back to the long text: fetched again.
    expect(run([long], s).channels[2]).toMatchObject({ description: "", descriptionHash: "01" });
  });
});

describe("private conversations by name", () => {
  const sync = { type: "ServerSync", message: create(pb.ServerSyncSchema, { session: 1 }) } as MumbleMessage;
  const dmFrom = (actor: number, text: string): MumbleMessage => ({
    type: "TextMessage",
    message: create(pb.TextMessageSchema, { actor, session: [1], message: text }),
  });

  it("stores the partner's name on private messages", () => {
    let s = run([sync, userState({ session: 1, name: "me" }), userState({ session: 7, name: "bob" }), dmFrom(7, "hi")]);
    s = addOwnMessage(s, { session: 7 }, "hey bob", 2);
    expect(s.messages.map((m) => m.peer)).toEqual(["bob", "bob"]);
    expect(s.privateCounts).toEqual({ bob: 1 });
  });

  it("gives a reused session number a fresh conversation", () => {
    const s = run([
      sync,
      userState({ session: 7, name: "bob" }),
      dmFrom(7, "hi from bob"),
      { type: "UserRemove", message: create(pb.UserRemoveSchema, { session: 7 }) },
      userState({ session: 7, name: "carol" }),
    ]);
    expect(privateMessagesWith(s, "carol")).toEqual([]);
    expect(s.privateCounts.carol).toBeUndefined();
    expect(privateMessagesWith(s, "bob").map((m) => m.text)).toEqual(["hi from bob"]);
  });

  it("keeps counting private messages after the message cap", () => {
    const s = Array.from({ length: MAX_MESSAGES + 10 }, (_, i) => dmFrom(7, `m${i}`)).reduce(
      (acc, m, i) => reduce(acc, m, i),
      run([sync, userState({ session: 7, name: "bob" })]),
    );
    expect(s.privateCounts.bob).toBe(MAX_MESSAGES + 10);
  });

  it("leaves channel messages without a peer", () => {
    const s = run([{ type: "TextMessage", message: create(pb.TextMessageSchema, { channelId: [0], message: "x" }) }]);
    expect(s.messages[0]?.peer).toBeNull();
  });
});

describe("permission denied", () => {
  it("keeps the last refusal as a notice", () => {
    const s = run([
      {
        type: "PermissionDenied",
        message: create(pb.PermissionDeniedSchema, { type: pb.PermissionDenied_DenyType.Permission, reason: "" }),
      },
    ]);
    expect(s.notice).toMatchObject({ text: "The server did not allow this.", at: 1000 });
  });

  it("uses the server's reason when there is one", () => {
    const s = run([
      { type: "PermissionDenied", message: create(pb.PermissionDeniedSchema, { reason: "No self-registration here" }) },
    ]);
    expect(s.notice?.text).toBe("No self-registration here");
  });
});

describe("root permissions", () => {
  it("keeps the permissions from ServerSync, unknown before it", () => {
    expect(initialState.rootPermissions).toBeNull();
    const s = run([
      {
        type: "ServerSync",
        message: create(pb.ServerSyncSchema, { session: 3, permissions: 0x2n | 0x4n }),
      },
    ]);
    expect(s.rootPermissions).toBe(0x6);
  });
});

describe("root permissions, later", () => {
  it("stays unknown when the server leaves the field out", () => {
    const s = run([{ type: "ServerSync", message: create(pb.ServerSyncSchema, { session: 3 }) }]);
    expect(s.rootPermissions).toBeNull();
  });

  it("follows permission updates for root", () => {
    const s = run([
      { type: "ServerSync", message: create(pb.ServerSyncSchema, { session: 3, permissions: 0x6n }) },
      { type: "PermissionQuery", message: create(pb.PermissionQuerySchema, { channelId: 5, permissions: 0xe }) },
    ]);
    expect(s.rootPermissions).toBe(0x6); // another channel
    const t = run([
      { type: "PermissionQuery", message: create(pb.PermissionQuerySchema, { channelId: 0, permissions: 0xe, flush: true }) },
    ], s);
    expect(t.rootPermissions).toBe(0xe);
  });
});

describe("channel permissions", () => {
  const pq = (channelId: number, permissions: number, flush = false): MumbleMessage => ({
    type: "PermissionQuery",
    message: create(pb.PermissionQuerySchema, { channelId, permissions, flush }),
  });

  it("keeps the answers per channel; a flush forgets the others", () => {
    let s = run([pq(5, 0x41), pq(6, 0x1)]);
    expect(s.channelPermissions).toEqual({ 5: 0x41, 6: 0x1 });
    s = run([pq(5, 0x4, true)], s);
    expect(s.channelPermissions).toEqual({ 5: 0x4 });
  });
});

describe("avatars (textures)", () => {
  const png = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);

  it("stores a short texture as it is", () => {
    const s = run([userState({ session: 2, name: "a", texture: png })]);
    expect(s.users[2]?.texture).toEqual(png);
    expect(s.users[2]?.textureHash).toBeNull();
  });

  it("stores the hash of a long one, then the data from the blob answer", () => {
    let s = run([userState({ session: 2, name: "a", textureHash: new Uint8Array([0xab, 0xcd]) })]);
    expect(s.users[2]?.textureHash).toBe("abcd");
    expect(s.users[2]?.texture).toBeNull();
    const long = new Uint8Array(200).fill(7); // blob answers are long
    s = run([userState({ session: 2, texture: long })], s);
    expect(s.users[2]?.texture).toEqual(long);
    expect(s.users[2]?.textureHash).toBe("abcd");
    // The same hash again (another state change) keeps the data.
    s = run([userState({ session: 2, textureHash: new Uint8Array([0xab, 0xcd]) })], s);
    expect(s.users[2]?.texture).toEqual(long);
    // A new picture: the old data goes.
    s = run([userState({ session: 2, textureHash: new Uint8Array([0x01]) })], s);
    expect(s.users[2]?.texture).toBeNull();
  });

  it("forgets the old hash when a short picture comes inline", () => {
    let s = run([userState({ session: 2, name: "a", textureHash: new Uint8Array([0xab]) })]);
    s = run([userState({ session: 2, texture: new Uint8Array(200) })], s); // blob answer
    s = run([userState({ session: 2, texture: png })], s); // a new short picture
    expect(s.users[2]?.textureHash).toBeNull();
    // The long one again: its hash is new to us, so it loads again.
    s = run([userState({ session: 2, textureHash: new Uint8Array([0xab]) })], s);
    expect(s.users[2]?.texture).toBeNull();
  });

  it("keeps its own copy of the bytes", () => {
    const frame = new Uint8Array([9, 0x89, 0x50, 0x4e, 0x47, 9]);
    const s = run([userState({ session: 2, name: "a", texture: frame.subarray(1, 5) })]);
    expect(s.users[2]?.texture?.buffer.byteLength).toBe(4);
  });

  it("forgets a removed picture (empty texture)", () => {
    let s = run([userState({ session: 2, name: "a", textureHash: new Uint8Array([1]) })]);
    s = run([userState({ session: 2, texture: new Uint8Array() })], s);
    expect(s.users[2]?.texture).toBeNull();
    expect(s.users[2]?.textureHash).toBeNull();
  });
});

describe("ban list and registered users", () => {
  const v4 = new Uint8Array([0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 203, 0, 113, 5]);
  it("stores the ban list", () => {
    const s = run([
      {
        type: "BanList",
        message: create(pb.BanListSchema, {
          bans: [{ address: v4, mask: 120, name: "troll", hash: "ab", reason: "spam", start: "2026-10-15", duration: 0 }],
        }),
      } as MumbleMessage,
    ]);
    expect(s.bans).toEqual([{ address: v4, mask: 120, name: "troll", hash: "ab", reason: "spam", start: "2026-10-15", duration: 0 }]);
    expect(banAddress(s.bans![0]!)).toBe("203.0.113.5/24");
  });

  it("stores the registered users", () => {
    const s = run([
      {
        type: "UserList",
        message: create(pb.UserListSchema, { users: [{ userId: 4, name: "alice" }, { userId: 0, name: "SuperUser" }] }),
      } as MumbleMessage,
    ]);
    expect(s.registered).toEqual([{ userId: 0, name: "SuperUser" }, { userId: 4, name: "alice" }]);
  });
});

describe("comments", () => {
  const long = "a".repeat(200);

  it("stores a short comment as it is", () => {
    const s = run([userState({ session: 2, name: "a", comment: "hi" })]);
    expect(s.users[2]).toMatchObject({ comment: "hi", commentHash: null });
  });

  it("stores the hash of a long one, then the text from the blob answer", () => {
    let s = run([userState({ session: 2, name: "a", commentHash: new Uint8Array([0xab]) })]);
    expect(s.users[2]).toMatchObject({ comment: "", commentHash: "ab" });
    s = run([userState({ session: 2, comment: long })], s);
    expect(s.users[2]).toMatchObject({ comment: long, commentHash: "ab" });
    // The same hash again keeps the text; a new one drops it.
    s = run([userState({ session: 2, commentHash: new Uint8Array([0xab]) })], s);
    expect(s.users[2]?.comment).toBe(long);
    s = run([userState({ session: 2, commentHash: new Uint8Array([0x01]) })], s);
    expect(s.users[2]).toMatchObject({ comment: "", commentHash: "01" });
  });

  it("forgets the hash when a short or empty comment comes inline", () => {
    let s = run([userState({ session: 2, name: "a", commentHash: new Uint8Array([0xab]) })]);
    s = run([userState({ session: 2, comment: "short" })], s);
    expect(s.users[2]).toMatchObject({ comment: "short", commentHash: null });
    s = run([userState({ session: 2, comment: "" })], s);
    expect(s.users[2]).toMatchObject({ comment: "", commentHash: null });
  });

  it("keeps the comment over other changes", () => {
    let s = run([userState({ session: 2, name: "a", comment: "hi" })]);
    s = run([userState({ session: 2, selfMute: true })], s);
    expect(s.users[2]?.comment).toBe("hi");
  });
});
