import { describe, expect, it } from "vitest";
import { channelTree, messagesIn, usersIn, usersSorted } from "../src/selectors";
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
