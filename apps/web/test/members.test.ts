import { describe, expect, it } from "vitest";
import { bioRequest, hiddenMembers, offlineMembers, onlineRows } from "../src/lib/members";
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

  it("drop broken input", () => {
    expect(parseServerEvent('{"type":"member","id":"7","name":"bob"}')).toBeNull();
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
