import { describe, expect, it } from "vitest";
import { initialState, Perm, type ServerState, type User } from "@kallisti/protocol";
import { describeRecording, mayRecord, parseRecordings, recordingUrl } from "../src/lib/recording";
import { initialTextState, manageRequest, parseServerEvent, reduceText, type TextState } from "../src/lib/textChannels";

const wire = {
  id: "0123456789abcdef",
  channel: 2,
  channelName: "Lounge",
  by: "mod",
  started: "2026-10-19T15:30:00Z",
  stopped: "2026-10-19T16:35:30Z",
  expires: "2026-10-26T16:35:30Z",
  reason: "stopped",
  size: 24_117_248,
  tracks: [
    { name: "alice", file: "alice.opus", channels: 1, start: 0 },
    { name: "bob", file: "bob.opus", channels: 2, start: 1500 },
  ],
};

describe("recording events", () => {
  it("reads the recorded channels from ready and later changes", () => {
    const ready = parseServerEvent(JSON.stringify({ type: "ready", channels: [], recording: [2] }));
    let state = reduceText(initialTextState, ready!);
    expect(state.recording).toEqual([2]);
    state = reduceText(state, parseServerEvent(JSON.stringify({ type: "recording", channels: [2, 7] }))!);
    expect(state.recording).toEqual([2, 7]);
    expect(parseServerEvent(JSON.stringify({ type: "recording", channels: ["x"] }))).toBeNull();
  });

  it("has no recording without the server's support", () => {
    const ready = parseServerEvent(JSON.stringify({ type: "ready", channels: [] }));
    expect(reduceText({ ...initialTextState, recording: [3] }, ready!).recording).toBeNull();
  });

  it("keeps the finished recordings, with times as numbers", () => {
    const ev = parseServerEvent(JSON.stringify({ type: "recordings", recordings: [wire] }));
    const state = reduceText(initialTextState, ev!);
    expect(state.recordings).toEqual([
      { ...wire, started: Date.parse(wire.started), stopped: Date.parse(wire.stopped), expires: Date.parse(wire.expires) },
    ]);
    expect(parseServerEvent(JSON.stringify({ type: "recordings", recordings: [{ ...wire, started: "never" }] }))).toBeNull();
    expect(parseServerEvent(JSON.stringify({ type: "recording-token", token: "t" }))).toEqual({ type: "recording-token", token: "t" });
  });

  it("asks the server to start and stop, for the list and a token", () => {
    expect(JSON.parse(manageRequest({ record: 5, on: true }))).toEqual({ type: "start-recording", voice: 5 });
    expect(JSON.parse(manageRequest({ record: 5, on: false }))).toEqual({ type: "stop-recording", voice: 5 });
    expect(JSON.parse(manageRequest({ recordings: true }))).toEqual({ type: "recordings" });
    expect(JSON.parse(manageRequest({ recordingToken: true }))).toEqual({ type: "recording-token" });
  });
});

describe("mayRecord", () => {
  const text: TextState = { ...initialTextState, voiceChats: true, recording: [], roles: { admins: [3], moderators: [5] } };
  const as = (userId: number | null, rootPermissions = 0): ServerState => ({
    ...initialState,
    rootPermissions,
    session: 1,
    users: { 1: { session: 1, name: "me", userId } as User },
  });

  it("lets admins and moderators record", () => {
    expect(mayRecord(as(5), text)).toBe(true);
    expect(mayRecord(as(3), text)).toBe(true);
    expect(mayRecord(as(0), text)).toBe(true); // SuperUser
    expect(mayRecord(as(8, Perm.Write), text)).toBe(true); // an admin by ACL
  });

  it("keeps others out, and everyone without the server's support", () => {
    expect(mayRecord(as(7), text)).toBe(false);
    expect(mayRecord(as(null), text)).toBe(false);
    expect(mayRecord(as(5), { ...text, recording: null })).toBe(false);
  });
});

describe("recordings for download", () => {
  const r = parseRecordings([wire])![0]!;

  it("describes one", () => {
    expect(describeRecording(r)).toEqual({ length: "1:05:30", size: "23.0 MB", speakers: "alice, bob" });
    expect(describeRecording({ ...r, tracks: [] }).speakers).toBe("nobody spoke");
  });

  it("makes the download link", () => {
    expect(recordingUrl("https://chat.example", r.id, "a b")).toBe("https://chat.example/api/recordings/0123456789abcdef?token=a%20b");
  });
});
