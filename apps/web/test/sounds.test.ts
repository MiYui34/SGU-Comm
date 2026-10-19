import { describe, expect, it } from "vitest";
import { initialState, type ServerState, type User } from "@kallisti/protocol";
import { allowSound, SOUND_GAP_MS, snapshotOf, voiceSound, type VoiceSnapshot } from "../src/lib/sounds";

const at = (channels: Record<number, number>, extra: Partial<VoiceSnapshot> = {}): VoiceSnapshot => ({
  me: 1,
  voice: channels[1] === 0 ? null : (channels[1] ?? null),
  users: Object.fromEntries(Object.entries(channels).map(([s, channelId]) => [s, { channelId }])),
  selfMute: false,
  selfDeaf: false,
  ...extra,
});

describe("voiceSound", () => {
  it("plays nothing for the first snapshot (the sync after login or a reconnect)", () => {
    expect(voiceSound(null, at({ 1: 5, 2: 5, 3: 5 }))).toBeNull();
  });

  it("plays join and leave for others in our voice channel", () => {
    expect(voiceSound(at({ 1: 5 }), at({ 1: 5, 2: 5 }))).toBe("join");
    expect(voiceSound(at({ 1: 5, 2: 6 }), at({ 1: 5, 2: 5 }))).toBe("join");
    expect(voiceSound(at({ 1: 5, 2: 5 }), at({ 1: 5, 2: 6 }))).toBe("leave");
    expect(voiceSound(at({ 1: 5, 2: 5 }), at({ 1: 5 }))).toBe("leave"); // disconnected
  });

  it("plays one sound for many at once", () => {
    expect(voiceSound(at({ 1: 5 }), at({ 1: 5, 2: 5, 3: 5, 4: 5 }))).toBe("join");
  });

  it("plays nothing for others outside our channel, or outside voice", () => {
    expect(voiceSound(at({ 1: 5, 2: 6 }), at({ 1: 5, 2: 7 }))).toBeNull();
    expect(voiceSound(at({ 1: 5 }), at({ 1: 5, 2: 0 }))).toBeNull();
    expect(voiceSound(at({ 1: 0 }), at({ 1: 0, 2: 0 }))).toBeNull(); // the lobby
  });

  it("plays our own join (also a move) and leave, not the others already there", () => {
    expect(voiceSound(at({ 1: 0, 2: 5 }), at({ 1: 5, 2: 5 }))).toBe("selfJoin");
    expect(voiceSound(at({ 1: 6, 2: 5 }), at({ 1: 5, 2: 5 }))).toBe("selfJoin");
    expect(voiceSound(at({ 1: 5, 2: 5 }), at({ 1: 0, 2: 5 }))).toBe("selfLeave");
  });

  it("plays mute and deafen changes, deafen first", () => {
    expect(voiceSound(at({ 1: 5 }), at({ 1: 5 }, { selfMute: true }))).toBe("mute");
    expect(voiceSound(at({ 1: 5 }, { selfMute: true }), at({ 1: 5 }))).toBe("unmute");
    expect(voiceSound(at({ 1: 5 }), at({ 1: 5 }, { selfMute: true, selfDeaf: true }))).toBe("deafen");
    expect(voiceSound(at({ 1: 5 }, { selfMute: true, selfDeaf: true }), at({ 1: 5 }))).toBe("undeafen");
  });

  it("plays nothing else while deafened", () => {
    const deaf = { selfMute: true, selfDeaf: true };
    expect(voiceSound(at({ 1: 5 }, deaf), at({ 1: 5, 2: 5 }, deaf))).toBeNull();
    expect(voiceSound(at({ 1: 0 }, deaf), at({ 1: 5 }, deaf))).toBeNull();
  });

  it("plays nothing when the session changed", () => {
    expect(voiceSound(at({ 1: 5 }), at({ 1: 5, 2: 6 }, { me: 2, voice: 6 }))).toBeNull();
  });
});

describe("snapshotOf", () => {
  const user = (session: number, channelId: number, extra: Partial<User> = {}) =>
    ({ session, channelId, selfMute: false, selfDeaf: false, ...extra }) as User;
  const state = (s: Partial<ServerState>): ServerState => ({ ...initialState, ...s });

  it("is null before we have a session", () => {
    expect(snapshotOf(state({ users: { 1: user(1, 0) } }))).toBeNull();
  });

  it("knows our voice channel: none in a lobby root", () => {
    const users = { 1: user(1, 0, { selfMute: true }), 2: user(2, 5) };
    expect(snapshotOf(state({ session: 1, users, rootPermissions: 0 }))).toEqual({
      me: 1,
      voice: null,
      users,
      selfMute: true,
      selfDeaf: false,
    });
    // Plain Mumble: root is a voice channel.
    expect(snapshotOf(state({ session: 1, users, rootPermissions: null }))?.voice).toBe(0);
    expect(snapshotOf(state({ session: 2, users, rootPermissions: 0 }))?.voice).toBe(5);
  });
});

describe("allowSound", () => {
  it("allows one sound per gap", () => {
    expect(allowSound(-Infinity, 0)).toBe(true);
    expect(allowSound(1000, 1000 + SOUND_GAP_MS - 1)).toBe(false);
    expect(allowSound(1000, 1000 + SOUND_GAP_MS)).toBe(true);
  });
});
