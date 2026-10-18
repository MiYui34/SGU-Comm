import { describe, expect, it } from "vitest";
import { bioRequest, offlineMembers } from "../src/lib/members";
import { initialTextState, parseServerEvent, reduceText } from "../src/lib/textChannels";

describe("offlineMembers", () => {
  const members = { 5: "zoe", 6: "Amy", 7: "bob", 8: "carl" };

  it("lists registered users who are not online, by name", () => {
    expect(offlineMembers(members, [{ userId: 7 }, { userId: null }])).toEqual([
      { id: 6, name: "Amy" },
      { id: 8, name: "carl" },
      { id: 5, name: "zoe" },
    ]);
  });

  it("matches by user id, not by name", () => {
    // A guest named like an offline user does not hide them.
    const guest = { userId: null, name: "bob" };
    const renamed = { userId: 7, name: "robert" };
    expect(offlineMembers({ 7: "bob" }, [guest])).toEqual([{ id: 7, name: "bob" }]);
    expect(offlineMembers({ 7: "bob" }, [renamed])).toEqual([]);
  });

  it("is empty without members", () => {
    expect(offlineMembers({}, [])).toEqual([]);
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
