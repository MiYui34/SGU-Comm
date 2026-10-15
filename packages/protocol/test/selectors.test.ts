import { describe, expect, it } from "vitest";
import {
  audioFormat,
  channelSiblings,
  channelTree,
  inVoice,
  isLobby,
  may,
  messagesIn,
  Perm,
  missingDescriptions,
  missingTextures,
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


describe("lobby and voice", () => {
  const SPEAK = 0x8;
  const base = (rootPermissions: number | null, channelId: number) => ({
    ...withData([ch(0, null, "Root"), ch(1, 0, "General")], [user(5, "me", channelId)]),
    session: 5,
    rootPermissions,
  });

  it("treats root as a lobby when nobody may speak there", () => {
    expect(isLobby(base(0x6, 0))).toBe(true);
    expect(isLobby(base(0x6 | SPEAK, 0))).toBe(false); // a plain Mumble server
    expect(isLobby(base(null, 0))).toBe(false); // not synced yet
  });

  it("is in voice outside the lobby only", () => {
    expect(inVoice(base(0x6, 0))).toBe(false);
    expect(inVoice(base(0x6, 1))).toBe(true);
    expect(inVoice(base(0x6 | SPEAK, 0))).toBe(true); // root is a voice channel there
    expect(inVoice({ ...base(0x6, 1), session: null })).toBe(false);
  });
});

describe("may", () => {
  it("answers from the known permissions, false when unknown", () => {
    const s = { ...initialState, rootPermissions: Perm.MakeChannel | Perm.Write, channelPermissions: { 5: Perm.Enter } };
    expect(may(s, 0, Perm.MakeChannel)).toBe(true);
    expect(may(s, 5, Perm.Write)).toBe(false);
    expect(may(s, 6, Perm.Enter)).toBe(false);
  });
});

describe("channelSiblings", () => {
  it("lists a channel and its siblings as the tree orders them", () => {
    const s = withData([ch(0, null, "Root"), ch(1, 0, "b", 1), ch(2, 0, "a", 1), ch(3, 0, "z", 0), ch(4, 1, "deep")]);
    expect(channelSiblings(s, 2).map((c) => c.id)).toEqual([3, 2, 1]);
    expect(channelSiblings(s, 4).map((c) => c.id)).toEqual([4]);
    expect(channelSiblings(s, 0)).toEqual([]); // root has none
  });
});

describe("missingTextures", () => {
  it("lists users whose picture is known only by its hash", () => {
    const s: ServerState = {
      ...initialState,
      users: {
        1: { ...emptyUser(1), textureHash: "ab" },
        2: { ...emptyUser(2), textureHash: "cd", texture: new Uint8Array([1]) },
        3: emptyUser(3),
      },
    };
    expect(missingTextures(s)).toEqual([1]);
  });
});
