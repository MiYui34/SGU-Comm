import { describe, expect, it } from "vitest";
import { accessChannel, memberCandidates, membersAction, privateAction } from "../src/lib/privateChannels";
import { initialTextState, manageRequest, parseServerEvent, reduceText } from "../src/lib/textChannels";

const text = [
  { id: 1, name: "general", mumble: 10 },
  { id: 2, name: "secret", mumble: 11 },
  { id: 3, name: "old" },
];

describe("accessChannel", () => {
  it("is a voice channel itself, or a text channel's mirror", () => {
    expect(accessChannel("voice", 4, text)).toBe(4);
    expect(accessChannel("text", 2, text)).toBe(11);
    expect(accessChannel("text", 3, text)).toBeNull();
    expect(accessChannel("text", 9, text)).toBeNull();
  });
});

describe("memberCandidates", () => {
  it("offers registered users who are not members yet, by name (not SuperUser)", () => {
    const registered = [
      { userId: 0, name: "SuperUser" },
      { userId: 5, name: "zoe" },
      { userId: 6, name: "amy" },
      { userId: 7, name: "bob" },
    ];
    expect(memberCandidates(registered, [{ id: 7, name: "bob" }])).toEqual([
      { id: 6, name: "amy" },
      { id: 5, name: "zoe" },
    ]);
    expect(memberCandidates(null, [])).toEqual([]);
  });
});

describe("membersAction", () => {
  const bob = { id: 7, name: "bob" };
  const amy = { id: 6, name: "amy" };
  it("sends nothing without a change", () => {
    expect(membersAction(11, [bob, amy], [amy, bob])).toBeNull();
  });
  it("sends the new members, sorted", () => {
    expect(membersAction(11, [bob], [bob, amy])).toEqual({ setPrivate: 11, on: true, users: [6, 7] });
    expect(membersAction(11, [bob], [])).toEqual({ setPrivate: 11, on: true, users: [] });
  });
});

describe("privateAction", () => {
  it("turns it on or off, keeping the members", () => {
    expect(privateAction(11, false, false, [])).toBeNull();
    expect(privateAction(11, true, true, [])).toBeNull();
    expect(privateAction(11, false, true, [])).toEqual({ setPrivate: 11, on: true, users: [] });
    expect(privateAction(11, true, false, [{ id: 7, name: "bob" }])).toEqual({ setPrivate: 11, on: false, users: [] });
  });
});

describe("private channel events", () => {
  it("come with ready and on their own", () => {
    const ready = parseServerEvent(
      JSON.stringify({ type: "ready", channels: [], private: [{ channel: 11, members: [{ id: 7, name: "bob" }] }, { channel: 4 }] }),
    );
    const state = reduceText(initialTextState, ready!);
    expect(state.private).toEqual({ 11: [{ id: 7, name: "bob" }], 4: [] });
    const ev = parseServerEvent(JSON.stringify({ type: "private", channels: [] }));
    expect(reduceText(state, ev!).private).toEqual({});
  });
  it("drop bad entries", () => {
    expect(parseServerEvent(JSON.stringify({ type: "private", channels: [{ channel: "x" }] }))).toBeNull();
    expect(parseServerEvent(JSON.stringify({ type: "private", channels: [{ channel: 2, members: [{ id: 1 }] }] }))).toBeNull();
    const ready = parseServerEvent(JSON.stringify({ type: "ready", channels: [], private: "x" }));
    expect(reduceText(initialTextState, ready!).private).toEqual({});
  });
  it("are asked for with set-private, create-channel and create-private-voice", () => {
    expect(JSON.parse(manageRequest({ setPrivate: 11, on: true, users: [7] }))).toEqual({ type: "set-private", voice: 11, on: true, users: [7] });
    expect(JSON.parse(manageRequest({ create: "club", private: true }))).toEqual({ type: "create-channel", name: "club", private: true });
    expect(JSON.parse(manageRequest({ createPrivateVoice: "Lounge" }))).toEqual({ type: "create-private-voice", name: "Lounge" });
  });
});
