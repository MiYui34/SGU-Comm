import { isFieldSet } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import * as pb from "../src/gen/Mumble_pb";
import { decodeFrames } from "../src/framing";
import { decodeMessage } from "../src/messages";
import {
  createChannel,
  editChannel,
  escapeHtml,
  joinChannel,
  queryPermissions,
  queryBans,
  queryRegistered,
  registerSelf,
  renameRegistered,
  setBans,
  unregister,
  removeChannel,
  requestDescriptions,
  requestTextures,
  sendText,
  setSelfState,
  setSelfTexture,
  textMessageInit,
} from "../src/actions";

const decodeOne = (bytes: Uint8Array) => {
  const { frames } = decodeFrames(bytes);
  expect(frames).toHaveLength(1);
  return decodeMessage(frames[0]!);
};

describe("escapeHtml", () => {
  it("escapes characters that have meaning in HTML", () => {
    expect(escapeHtml(`<b>"Tom" & 'Jerry'</b>`)).toBe(
      "&lt;b&gt;&quot;Tom&quot; &amp; &#39;Jerry&#39;&lt;/b&gt;",
    );
  });
});

describe("sendText", () => {
  it("sends escaped text to one channel", () => {
    const msg = decodeOne(sendText({ channelId: 4 }, "a < b"));
    if (msg.type !== "TextMessage") throw new Error();
    expect(msg.message.channelId).toEqual([4]);
    expect(msg.message.session).toEqual([]);
    expect(msg.message.message).toBe("a &lt; b");
  });

  it("sends a private message to one user", () => {
    const msg = decodeOne(sendText({ session: 9 }, "psst"));
    if (msg.type !== "TextMessage") throw new Error();
    expect(msg.message.session).toEqual([9]);
    expect(msg.message.channelId).toEqual([]);
  });
});

describe("joinChannel", () => {
  it("moves the own session to a channel", () => {
    const msg = decodeOne(joinChannel(9, 0));
    if (msg.type !== "UserState") throw new Error();
    expect(msg.message).toMatchObject({ session: 9, channelId: 0 });
  });
});

describe("setSelfState", () => {
  it("sends self mute and self deaf", () => {
    const msg = decodeOne(setSelfState(9, { selfMute: true, selfDeaf: false }));
    if (msg.type !== "UserState") throw new Error();
    expect(msg.message).toMatchObject({ session: 9, selfMute: true, selfDeaf: false });
  });
});

describe("requestDescriptions", () => {
  it("asks for channel description blobs", () => {
    const msg = decodeOne(requestDescriptions([0, 4]));
    if (msg.type !== "RequestBlob") throw new Error();
    expect(msg.message.channelDescription).toEqual([0, 4]);
  });
});

describe("registerSelf", () => {
  it("asks Murmur to register the own session", () => {
    const msg = decodeOne(registerSelf(12));
    if (msg.type !== "UserState") throw new Error();
    expect(msg.message).toMatchObject({ session: 12, userId: 0 });
  });
});

describe("channel management", () => {
  it("creates a channel below a parent", () => {
    const msg = decodeOne(createChannel(0, "Games", { temporary: true }));
    if (msg.type !== "ChannelState") throw new Error();
    expect(msg.message.parent).toBe(0);
    expect(msg.message.name).toBe("Games");
    expect(msg.message.temporary).toBe(true);
    expect(msg.message.channelId).toBe(0); // not set: a new channel
  });

  it("edits only the given fields", () => {
    const msg = decodeOne(editChannel(5, { name: "Lounge", position: 2 }));
    if (msg.type !== "ChannelState") throw new Error();
    expect(msg.message.channelId).toBe(5);
    expect(msg.message.name).toBe("Lounge");
    expect(msg.message.position).toBe(2);
    expect(msg.message.description).toBe("");
  });

  it("removes a channel and asks for permissions", () => {
    const rm = decodeOne(removeChannel(5));
    expect(rm.type === "ChannelRemove" && rm.message.channelId).toBe(5);
    const q = decodeOne(queryPermissions(5));
    expect(q.type === "PermissionQuery" && q.message.channelId).toBe(5);
  });
});

describe("textures", () => {
  it("asks for avatar blobs and sets our own", () => {
    const ask = decodeOne(requestTextures([3, 4]));
    if (ask.type !== "RequestBlob") throw new Error();
    expect(ask.message.sessionTexture).toEqual([3, 4]);
    const set = decodeOne(setSelfTexture(7, new Uint8Array([1, 2])));
    if (set.type !== "UserState") throw new Error();
    expect(set.message.session).toBe(7);
    expect([...set.message.texture]).toEqual([1, 2]);
  });
});

describe("ban list and registered users", () => {
  it("queries and changes them", () => {
    const q = decodeOne(queryBans());
    if (q.type !== "BanList") throw new Error();
    expect(q.message.query).toBe(true);
    const ban = { address: new Uint8Array(16), mask: 128, name: "x", hash: "h", reason: "r", start: "", duration: 0 };
    const set = decodeOne(setBans([ban]));
    if (set.type !== "BanList") throw new Error();
    expect(set.message.bans).toHaveLength(1);
    expect(set.message.query).toBe(false);
    const users = decodeOne(queryRegistered());
    if (users.type !== "UserList") throw new Error();
    expect(users.message.users).toHaveLength(0);
    const rename = decodeOne(renameRegistered(4, "bob"));
    if (rename.type !== "UserList") throw new Error();
    expect(rename.message.users[0]).toMatchObject({ userId: 4, name: "bob" });
    const gone = decodeOne(unregister(4));
    if (gone.type !== "UserList") throw new Error();
    expect(gone.message.users[0]?.userId).toBe(4);
    expect(isFieldSet(gone.message.users[0]!, pb.UserList_UserSchema.field.name)).toBe(false);
  });
});

describe("textMessageInit", () => {
  it("sends line breaks as <br> (Mumble text is HTML)", () => {
    expect(textMessageInit({ session: 3 }, "a < b\nc").message).toBe("a &lt; b<br>c");
  });
});
