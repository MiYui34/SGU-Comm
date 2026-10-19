import { describe, expect, it } from "vitest";
import { emptyUser, initialState, type ServerState, type User } from "@kallisti/protocol";
import { listenOnlyAction, listenOnlySessions, speakerCandidates } from "../src/lib/listenOnly";
import { initialTextState, manageRequest, parseServerEvent, reduceText } from "../src/lib/textChannels";

const user = (session: number, name: string, userId: number | null): User => ({ ...emptyUser(session), name, userId });
const server = (...users: User[]): ServerState => ({
  ...initialState,
  users: Object.fromEntries(users.map((u) => [u.session, u])),
});

describe("speakerCandidates", () => {
  it("offers registered users who are online, by name (not SuperUser: it always may)", () => {
    const s = server(user(5, "SuperUser", 0), user(1, "zoe", 5), user(2, "jukebox", 9), user(3, "guest", null), user(4, "amy", 6));
    expect(speakerCandidates(s)).toEqual([
      { id: 6, name: "amy" },
      { id: 9, name: "jukebox" },
      { id: 5, name: "zoe" },
    ]);
  });
});

describe("listenOnlySessions", () => {
  it("finds who is in a listen-only channel (only speakers can speak there)", () => {
    const s = server({ ...user(1, "jukebox", 9), channelId: 4 }, { ...user(2, "amy", 6), channelId: 3 }, user(3, "zoe", 5));
    expect(listenOnlySessions(s, { 4: [] })).toEqual(new Set([1]));
    expect(listenOnlySessions(s, {})).toEqual(new Set());
  });
});

describe("listenOnlyAction", () => {
  const off = { on: false, speakers: [] };
  it("sends nothing without a change", () => {
    expect(listenOnlyAction(4, off, off)).toBeNull();
    const on = { on: true, speakers: [{ id: 9, name: "jukebox" }] };
    expect(listenOnlyAction(4, on, { ...on, speakers: [...on.speakers] })).toBeNull();
  });
  it("sends the new state", () => {
    expect(listenOnlyAction(4, off, { on: true, speakers: [{ id: 9, name: "jukebox" }] })).toEqual({ listenOnly: 4, on: true, users: [9] });
    expect(listenOnlyAction(4, { on: true, speakers: [{ id: 9, name: "jukebox" }] }, off)).toEqual({ listenOnly: 4, on: false, users: [] });
  });
  it("drops speakers when it is off", () => {
    expect(listenOnlyAction(4, off, { on: false, speakers: [{ id: 9, name: "jukebox" }] })).toBeNull();
  });
});

describe("listen-only events", () => {
  it("come with ready and on their own", () => {
    const ready = parseServerEvent(JSON.stringify({ type: "ready", channels: [], listenOnly: [{ channel: 4, speakers: [{ id: 9, name: "jukebox" }] }] }));
    const state = reduceText(initialTextState, ready!);
    expect(state.listenOnly).toEqual({ 4: [{ id: 9, name: "jukebox" }] });
    const ev = parseServerEvent(JSON.stringify({ type: "listen-only", channels: [] }));
    expect(reduceText(state, ev!).listenOnly).toEqual({});
  });
  it("drop bad entries", () => {
    const ev = parseServerEvent(JSON.stringify({ type: "listen-only", channels: [{ channel: "x" }, { channel: 2, speakers: [{ id: 1 }] }] }));
    expect(ev).toBeNull();
  });
  it("are asked for with set-listen-only", () => {
    expect(JSON.parse(manageRequest({ listenOnly: 4, on: true, users: [9] }))).toEqual({ type: "set-listen-only", voice: 4, on: true, users: [9] });
  });
});
