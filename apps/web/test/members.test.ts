import { describe, expect, it } from "vitest";
import { bioRequest, hiddenMembers, offlineMembers, onlineRows, pictureNames, readerGroups, type ReaderRow } from "../src/lib/members";
import { initialTextState, parseServerEvent, reduceText } from "../src/lib/textChannels";

describe("offlineMembers", () => {
  const members = { 5: "zoe", 6: "Amy", 7: "bob", 8: "carl" };

  it("lists registered users who are not online, by name", () => {
    expect(offlineMembers(members, {}, [{ userId: 7 }, { userId: null }])).toEqual([
      { id: 6, name: "Amy" },
      { id: 8, name: "carl" },
      { id: 5, name: "zoe" },
    ]);
  });

  it("matches by user id, not by name", () => {
    // A guest named like an offline user does not hide them.
    const guest = { userId: null, name: "bob" };
    const renamed = { userId: 7, name: "robert" };
    expect(offlineMembers({ 7: "bob" }, {}, [guest])).toEqual([{ id: 7, name: "bob" }]);
    expect(offlineMembers({ 7: "bob" }, {}, [renamed])).toEqual([]);
  });

  it("leaves out members online where we cannot see them", () => {
    // Also a new user whose online state comes before its user state.
    expect(offlineMembers(members, { 5: 12, 6: 13 }, [{ userId: 7 }])).toEqual([{ id: 8, name: "carl" }]);
  });

  it("is empty without members", () => {
    expect(offlineMembers({}, {}, [])).toEqual([]);
  });
});

describe("hiddenMembers", () => {
  it("lists members online in channels we cannot see, with their session, by name", () => {
    const members = { 5: "zoe", 6: "Amy", 7: "bob", 8: "carl" };
    expect(hiddenMembers(members, { 5: 12, 6: 13, 7: 14, 9: 15 }, [{ userId: 7 }])).toEqual([
      { id: 6, name: "Amy", session: 13 },
      { id: 5, name: "zoe", session: 12 },
    ]);
  });
});

describe("onlineRows", () => {
  it("merges users and hidden members by name", () => {
    const users = [
      { session: 1, name: "bob" },
      { session: 2, name: "dan" },
    ];
    const hidden = [
      { id: 6, name: "Amy", session: 13 },
      { id: 8, name: "carl", session: 14 },
    ];
    expect(onlineRows(users, hidden).map((r) => ("user" in r ? r.user.name : r.member.name))).toEqual(["Amy", "bob", "carl", "dan"]);
  });
});

describe("readerGroups", () => {
  type U = { session: number; name: string; userId: number | null };
  const label = (r: ReaderRow<U>) => ("user" in r ? `${r.user.name}@${r.user.session}` : "member" in r ? `${r.member.name}~${r.member.session}` : `${r.offline.name}-`);
  const titles = (gs: { title: string; rows: ReaderRow<U>[] }[] | null) => gs!.map((g) => [g.title, g.rows.map(label)]);
  const registered = { 0: "SuperUser", 5: "zoe", 6: "Amy", 7: "bob", 8: "carl", 9: "dan", 10: "eve" };
  const roles = { admins: [9], moderators: [10] };

  it("lists members and staff, each online first, then offline, by name", () => {
    const users: U[] = [
      { session: 1, name: "bob", userId: 7 },
      { session: 2, name: "eve", userId: 10 },
      { session: 3, name: "guest", userId: null },
      { session: 4, name: "carl", userId: 8 }, // sees the channel, no member
    ];
    const members = [
      { id: 5, name: "zoe" },
      { id: 6, name: "Amy" },
      { id: 7, name: "bob" },
    ];
    expect(titles(readerGroups(members, roles, registered, { 5: 12 }, users, 7))).toEqual([
      ["Members", ["bob@1", "zoe~12", "Amy-"]],
      ["Staff", ["eve@2", "dan-", "SuperUser-"]],
    ]);
  });

  it("lists staff who are members once, as staff", () => {
    const members = [
      { id: 9, name: "dan" },
      { id: 0, name: "SuperUser" },
    ];
    expect(titles(readerGroups(members, roles, { 9: "dan" }, {}, [], 0))).toEqual([["Staff", ["dan-", "SuperUser-"]]]);
  });

  it("is null for who is no member or staff (the server leaves the members out)", () => {
    const members = [{ id: 7, name: "bob" }];
    expect(readerGroups(members, roles, registered, {}, [], 8)).toBeNull();
    expect(readerGroups(members, roles, registered, {}, [], null)).toBeNull();
    expect(readerGroups([], roles, registered, {}, [], 7)).toBeNull();
    expect(readerGroups([], roles, registered, {}, [], 10)).not.toBeNull();
  });

  it("names members who are not in the member list (guest accounts) by the channel's list", () => {
    expect(titles(readerGroups([{ id: 30, name: "guest-1" }], { admins: [], moderators: [] }, {}, { 30: 40 }, [], 30))).toEqual([
      ["Members", ["guest-1~40"]],
    ]);
  });
});

describe("online events", () => {
  const ready = parseServerEvent('{"type":"ready","channels":[],"voiceChats":true,"online":{"5":12,"6":13}}')!;
  const s = reduceText(initialTextState, ready);

  it("come with ready", () => {
    expect(s.online).toEqual({ 5: 12, 6: 13 });
    expect(reduceText(s, parseServerEvent('{"type":"ready","channels":[]}')!).online).toEqual({});
  });

  it("change sessions and go offline", () => {
    const joined = reduceText(s, parseServerEvent('{"type":"online","id":7,"session":20}')!);
    expect(joined.online).toEqual({ 5: 12, 6: 13, 7: 20 });
    const left = reduceText(joined, parseServerEvent('{"type":"online","id":5}')!);
    expect(left.online).toEqual({ 6: 13, 7: 20 });
  });

  it("drop broken input", () => {
    expect(parseServerEvent('{"type":"online","id":"7"}')).toBeNull();
    expect(reduceText(initialTextState, parseServerEvent('{"type":"ready","channels":[],"online":{"5":"x"}}')!).online).toEqual({});
  });
});

describe("member events", () => {
  const ready = parseServerEvent('{"type":"ready","channels":[],"voiceChats":true,"members":[{"id":5,"name":"zoe"},{"id":6,"name":"amy"}]}')!;
  const s = reduceText(initialTextState, ready);

  it("come with ready", () => {
    expect(s.members).toEqual({ 5: "zoe", 6: "amy" });
    expect(reduceText(s, parseServerEvent('{"type":"ready","channels":[]}')!).members).toEqual({});
  });

  it("add, rename and remove users", () => {
    const added = reduceText(s, parseServerEvent('{"type":"member","id":7,"name":"bob"}')!);
    expect(added.members).toEqual({ 5: "zoe", 6: "amy", 7: "bob" });
    const renamed = reduceText(added, parseServerEvent('{"type":"member","id":7,"name":"robert"}')!);
    expect(renamed.members[7]).toBe("robert");
    const removed = reduceText(renamed, parseServerEvent('{"type":"member","id":5,"gone":true}')!);
    expect(removed.members).toEqual({ 6: "amy", 7: "robert" });
  });

  it("rename direct chats", () => {
    const withDirects = reduceText(
      s,
      parseServerEvent('{"type":"ready","channels":[],"voiceChats":true,"members":[{"id":7,"name":"bob"}],"directs":[{"channel":3,"peer":{"id":7,"name":"bob"}},{"channel":4,"peer":{"id":8,"name":"carl"}}]}')!,
    );
    const renamed = reduceText(withDirects, parseServerEvent('{"type":"member","id":7,"name":"robert"}')!);
    expect(renamed.directs).toEqual({ 3: { id: 7, name: "robert" }, 4: { id: 8, name: "carl" } });
    // A removed user's chat keeps its last name.
    expect(reduceText(renamed, parseServerEvent('{"type":"member","id":7,"gone":true}')!).directs[3]).toEqual({ id: 7, name: "robert" });
  });

  it("tell who has a saved picture", () => {
    const ready = parseServerEvent('{"type":"ready","channels":[],"members":[{"id":5,"name":"zoe","picture":true},{"id":6,"name":"amy"}]}')!;
    const s = reduceText(initialTextState, ready);
    expect(s.pictures).toEqual(new Set([5]));
    const set = reduceText(s, parseServerEvent('{"type":"member","id":6,"picture":true}')!);
    expect(set.pictures).toEqual(new Set([5, 6]));
    expect(set.members).toEqual(s.members);
    expect(pictureNames(set.members, set.pictures)).toEqual(new Set(["zoe", "amy"]));
    expect(reduceText(set, parseServerEvent('{"type":"member","id":5,"picture":false}')!).pictures).toEqual(new Set([6]));
    expect(reduceText(set, parseServerEvent('{"type":"member","id":6,"gone":true}')!).pictures).toEqual(new Set([5]));
    // Ids without a member (guest accounts) have no name.
    expect(pictureNames({ 5: "zoe" }, new Set([5, 9]))).toEqual(new Set(["zoe"]));
  });

  it("drop broken input", () => {
    expect(parseServerEvent('{"type":"member","id":"7","name":"bob"}')).toBeNull();
    expect(parseServerEvent('{"type":"member","id":7,"picture":1}')).toBeNull();
    expect(parseServerEvent('{"type":"member","id":7}')).toBeNull();
    const broken = parseServerEvent('{"type":"ready","channels":[],"members":[{"id":5}]}')!;
    expect(reduceText(initialTextState, broken).members).toEqual({});
  });
});

describe("bios", () => {
  it("are asked for and kept by user id", () => {
    expect(JSON.parse(bioRequest(7))).toEqual({ type: "bio", user: 7 });
    const s = reduceText(initialTextState, parseServerEvent('{"type":"bio","user":7,"text":"<b>hi</b>"}')!);
    expect(s.bios).toEqual({ 7: "<b>hi</b>" });
    expect(parseServerEvent('{"type":"bio","user":7}')).toBeNull();
  });
});
