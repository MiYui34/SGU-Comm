import { describe, expect, it } from "vitest";
import type { Channel, ServerState, User } from "@kallisti/protocol";
import { myChannelOf, myNameOf, voiceRooms } from "../src/sgu/fromServer";

const channel = (id: number, name: string, parent: number | null, position = 0): Channel => ({
  id,
  parent,
  name,
  description: "",
  descriptionHash: null,
  position,
  temporary: false,
});

const user = (session: number, name: string, channelId: number): User => ({
  session,
  name,
  channelId,
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

const stateOf = (channels: Channel[], users: User[], session: number | null = 7): ServerState => ({
  serverVersion: null,
  status: "synced",
  session,
  welcomeText: "",
  error: null,
  rejectType: null,
  channels: Object.fromEntries(channels.map((c) => [c.id, c])),
  users: Object.fromEntries(users.map((u) => [u.session, u])),
  messages: [],
  privateCounts: {},
  notice: null,
  rootPermissions: null,
  channelPermissions: {},
  bans: null,
  registered: null,
});

describe("voice rooms from the server", () => {
  it("lists the rooms below the lobby, and only the ones people can sit in", () => {
    const server = stateOf(
      [
        channel(0, "服务器", null),
        channel(1, "大厅", 0),
        channel(2, "分类", 0, 2),
        channel(3, "挖矿", 2, 1),
      ],
      [user(1, "Alice", 1), user(2, "Bob", 3)],
    );
    expect(voiceRooms(server).map((room) => [room.id, room.name, room.members])).toEqual([
      ["1", "大厅", ["Alice"]],
      ["3", "挖矿", ["Bob"]],
    ]);
  });

  it("sorts by position, then by name", () => {
    const server = stateOf(
      [
        channel(0, "服务器", null),
        channel(1, "乙", 0, 1),
        channel(2, "甲", 0, 1),
        channel(3, "丙", 0, 0),
      ],
      [],
    );
    expect(voiceRooms(server).map((room) => room.name)).toEqual(["丙", "甲", "乙"]);
  });

  it("puts everyone in a room in order, and nobody twice", () => {
    const server = stateOf(
      [channel(0, "服务器", null), channel(1, "主城", 0)],
      [user(1, "Zoe", 1), user(2, "Adam", 1)],
    );
    expect(voiceRooms(server)[0]?.members).toEqual(["Adam", "Zoe"]);
  });

  it("has no rooms before the server says what they are", () => {
    expect(voiceRooms(stateOf([], []))).toEqual([]);
  });
});

describe("who am I", () => {
  it("gives my name and the room I am in", () => {
    const server = stateOf(
      [channel(0, "服务器", null), channel(1, "主城", 0)],
      [user(7, "Kernel", 1)],
    );
    expect(myNameOf(server)).toBe("Kernel");
    expect(myChannelOf(server)).toBe("1");
  });

  it("says the lobby (0) when I am not in a room", () => {
    const server = stateOf([channel(0, "服务器", null)], [user(7, "Kernel", 0)]);
    expect(myChannelOf(server)).toBe("0");
  });

  it("knows nothing before the session is known", () => {
    const server = stateOf([channel(0, "服务器", null)], [], null);
    expect(myNameOf(server)).toBeNull();
    expect(myChannelOf(server)).toBeNull();
  });
});
