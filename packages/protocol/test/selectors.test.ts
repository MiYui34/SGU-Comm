import { describe, expect, it } from "vitest";
import {
  audioFormat,
  channelTree,
  messagesIn,
  missingDescriptions,
  privateMessagesWith,
  usersIn,
  usersSorted,
} from "../src/selectors";
import {
  emptyChannel,
  emptyUser,
  initialState,
  type Channel,
  type ServerState,
  type User,
} from "../src/state";

const ch = (id: number, parent: number | null, name: string, position = 0): Channel => ({
  ...emptyChannel(id),
  parent,
  name,
  position,
});
const user = (session: number, name: string, channelId: number): User => ({
  ...emptyUser(session),
  name,
  channelId,
});
const withData = (channels: Channel[], users: User[] = []): ServerState => ({
  ...initialState,
  channels: Object.fromEntries(channels.map((c) => [c.id, c])),
  users: Object.fromEntries(users.map((u) => [u.session, u])),
});

describe("channelTree", () => {
  it("returns null when there is no root channel", () => {
    expect(channelTree(initialState)).toBeNull();
  });

  it("nests channels and sorts by position, then name", () => {
    const tree = channelTree(
      withData([
        ch(0, null, "Root"),
        ch(1, 0, "Zeta"),
        ch(2, 0, "Alpha"),
        ch(3, 0, "First", -1),
        ch(4, 2, "Sub"),
      ]),
    );
    const names = (n: NonNullable<typeof tree>): unknown => [
      n.channel.name,
      n.children.map(names),
    ];
    expect(names(tree!)).toEqual([
      "Root",
      [
        ["First", []],
        ["Alpha", [["Sub", []]]],
        ["Zeta", []],
      ],
    ]);
  });
});

describe("channelTree with bad data", () => {
  it("does not loop forever on parent cycles", () => {
    const tree = channelTree(withData([ch(0, 0, "Root"), ch(1, 2, "A"), ch(2, 1, "B")]));
    expect(tree).toEqual({ channel: ch(0, 0, "Root"), children: [] });
  });
});

describe("usersIn", () => {
  it("returns the users of one channel sorted by name, case-insensitive", () => {
    const s = withData([ch(0, null, "Root")], [
      user(1, "zed", 0),
      user(2, "Amy", 0),
      user(3, "bob", 5),
      user(4, "carl", 0),
    ]);
    expect(usersIn(s, 0).map((u) => u.name)).toEqual(["Amy", "carl", "zed"]);
  });
});

describe("usersSorted", () => {
  it("returns every user sorted by name", () => {
    const s = withData([], [user(1, "zed", 0), user(2, "Amy", 3)]);
    expect(usersSorted(s).map((u) => u.name)).toEqual(["Amy", "zed"]);
  });
});

describe("messagesIn", () => {
  const msg = (text: string, channelIds: number[], treeIds: number[] = []) => ({
    actor: null,
    actorName: null,
    channelIds,
    treeIds,
    sessions: [],
    text,
    at: 0,
    peer: null,
  });

  it("returns messages sent to the channel or to a tree that contains it", () => {
    const s: ServerState = {
      ...withData([ch(0, null, "Root"), ch(1, 0, "A"), ch(2, 1, "B")]),
      messages: [
        msg("to B", [2]),
        msg("to A", [1]),
        msg("tree A", [], [1]),
        msg("tree root", [], [0]),
        msg("tree B", [], [2]),
      ],
    };
    expect(messagesIn(s, 2).map((m) => m.text)).toEqual([
      "to B",
      "tree A",
      "tree root",
      "tree B",
    ]);
    expect(messagesIn(s, 1).map((m) => m.text)).toEqual(["to A", "tree A", "tree root"]);
  });
});

describe("audioFormat", () => {
  const v = (major: number, minor: number) => ({ ...initialState, serverVersion: { major, minor, patch: 0 } });

  it("uses protobuf audio from 1.5 on", () => {
    expect(audioFormat(v(1, 5))).toBe("protobuf");
    expect(audioFormat(v(2, 0))).toBe("protobuf");
  });

  it("uses legacy audio before 1.5 and when the version is unknown", () => {
    expect(audioFormat(v(1, 4))).toBe("legacy");
    expect(audioFormat(initialState)).toBe("legacy");
  });
});

describe("privateMessagesWith", () => {
  const msg = (text: string, peer: string | null) => ({
    actor: null,
    actorName: null,
    channelIds: [],
    treeIds: [],
    sessions: [],
    text,
    at: 0,
    peer,
  });

  it("returns the private conversation with one person, by name", () => {
    const s: ServerState = {
      ...initialState,
      messages: [msg("from bob", "bob"), msg("to bob", "bob"), msg("to carol", "carol"), msg("channel", null)],
    };
    expect(privateMessagesWith(s, "bob").map((m) => m.text)).toEqual(["from bob", "to bob"]);
  });
});

