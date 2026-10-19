import { describe, expect, it } from "vitest";
import { initialState, type ServerState, type User } from "@kallisti/protocol";
import { allowSound, SOUND_GAP_MS, snapshotOf, voiceSound, type VoiceSnapshot } from "../src/lib/sounds";

// Channels are ignored now (no join or leave sounds); kept so the cases read
// as before.
const at = (_channels: Record<number, number>, extra: Partial<VoiceSnapshot> = {}): VoiceSnapshot => ({
  me: 1,
  selfMute: false,
  selfDeaf: false,
  ...extra,
});

describe("voiceSound", () => {
  it("plays nothing for the first snapshot (the sync after login or a reconnect)", () => {
    expect(voiceSound(null, at({ 1: 5, 2: 5, 3: 5 }))).toBeNull();
  });

  it("plays nothing for joins and leaves, ours or others'", () => {
    expect(voiceSound(at({ 1: 5 }), at({ 1: 5, 2: 5 }))).toBeNull();
    expect(voiceSound(at({ 1: 5, 2: 5 }), at({ 1: 5 }))).toBeNull();
    expect(voiceSound(at({ 1: 0, 2: 5 }), at({ 1: 5, 2: 5 }))).toBeNull();
    expect(voiceSound(at({ 1: 5, 2: 5 }), at({ 1: 0, 2: 5 }))).toBeNull();
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
    expect(voiceSound(at({ 1: 5 }), at({ 1: 5, 2: 6 }, { me: 2 }))).toBeNull();
  });
});

describe("snapshotOf", () => {
  const user = (session: number, channelId: number, extra: Partial<User> = {}) =>
    ({ session, channelId, selfMute: false, selfDeaf: false, ...extra }) as User;
  const state = (s: Partial<ServerState>): ServerState => ({ ...initialState, ...s });

  it("is null before we have a session", () => {
    expect(snapshotOf(state({ users: { 1: user(1, 0) } }))).toBeNull();
  });

  it("keeps our session and mute state", () => {
    const users = { 1: user(1, 0, { selfMute: true }), 2: user(2, 5) };
    expect(snapshotOf(state({ session: 1, users }))).toEqual({ me: 1, selfMute: true, selfDeaf: false });
  });
});

describe("allowSound", () => {
  it("allows one sound per gap", () => {
    expect(allowSound(-Infinity, 0)).toBe(true);
    expect(allowSound(1000, 1000 + SOUND_GAP_MS - 1)).toBe(false);
    expect(allowSound(1000, 1000 + SOUND_GAP_MS)).toBe(true);
  });
});
