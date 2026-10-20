import { describe, expect, it } from "vitest";
import { emptyUser, initialState, Perm, type Channel, type ServerState, type User } from "@kallisti/protocol";
import { badgeOf, channelModActions, channelModIn } from "../src/lib/roles";
import { canManageMembers, canModerate, menuRights } from "../src/lib/channelManagement";
import { channelTimedOutUntil, moderationRequest, userModeration } from "../src/lib/moderation";
import { initialTextState, manageRequest, parseServerEvent, reduceText, type TextState } from "../src/lib/textChannels";

const ch = (id: number, parent: number | null): Channel => ({
  id, parent, name: `c${id}`, description: "", descriptionHash: null, position: id, temporary: false,
});
const user = (session: number, userId: number | null, channelId = 1): User => ({ ...emptyUser(session), name: `u${session}`, userId, channelId });
// Channels: root (a lobby), 1 with subchannel 2, 3, and 4 (a mirror of
// #general). I am session 1 (user 5), a moderator of channel 1 and of
// #general; user 6 moderates channel 2.
const MOD = Perm.MuteDeafen | Perm.Move | Perm.Enter;
const server = (rootPerms = 0, perms: Record<number, number> = { 1: MOD, 2: MOD }): ServerState => ({
  ...initialState,
  session: 1,
  rootPermissions: rootPerms,
  channelPermissions: perms,
  channels: Object.fromEntries([ch(0, null), ch(1, 0), ch(2, 1), ch(3, 0), ch(4, 0)].map((c) => [c.id, c])),
  users: { 1: user(1, 5, 3), 2: user(2, null), 3: user(3, 6, 2), 4: user(4, 7) },
});
const text: TextState = {
  ...initialTextState,
  voiceChats: true,
  channels: [{ id: 10, name: "general", mumble: 4 }, { id: 11, name: "random" }],
  private: { 1: [] },
  roles: { admins: [], moderators: [7], channelMods: { 1: [5], 2: [6], 4: [5] } },
};

describe("channel moderators", () => {
  it("come with the roles, and moderate their channel and its subchannels", () => {
    const ready = parseServerEvent(
      '{"type":"ready","channels":[],"voiceChats":true,"roles":{"admins":[],"moderators":[],"channelModerators":[{"channel":1,"users":[5]}]}}',
    );
    const s = reduceText(initialTextState, ready!);
    expect(s.roles.channelMods).toEqual({ 1: [5] });
    const changed = parseServerEvent('{"type":"roles","admins":[],"moderators":[],"channelModerators":[]}');
    expect(reduceText(s, changed!).roles.channelMods).toEqual({});
    expect(channelModIn(server(), text.roles, 2, 5)).toBe(true);
    expect(channelModIn(server(), text.roles, 1, 6)).toBe(false);
    expect(channelModIn(server(), text.roles, 3, 5)).toBe(false);
    expect(channelModIn(server(), text.roles, null, 5)).toBe(false);
  });

  it("have a tag only in their channels' member lists; server roles come first", () => {
    expect(badgeOf(server(), text.roles, 2, 6)).toBe("channelmod");
    expect(badgeOf(server(), text.roles, 3, 6)).toBeNull();
    expect(badgeOf(server(), text.roles, 3, 7)).toBe("moderator");
    expect(badgeOf(server(), text.roles, 1, null)).toBeNull();
  });

  it("are appointed one request per change", () => {
    expect(channelModActions(1, [5, 6], [6, 8])).toEqual([
      { channelModerator: 8, channel: 1, on: true },
      { channelModerator: 5, channel: 1, on: false },
    ]);
    expect(JSON.parse(manageRequest({ channelModerator: 8, channel: 1, on: true }))).toEqual({ type: "set-channel-moderator", user: 8, voice: 1, on: true });
  });

  it("moderate and edit (topic and slow mode) only their channels", () => {
    expect(canModerate(server(), text, 2)).toBe(true);
    expect(canModerate(server(), text, 3)).toBe(false);
    expect(canModerate(server(Perm.MakeChannel), text, 3)).toBe(true); // staff
    expect(menuRights(server(), text, "text", 10)?.canEdit).toBe(true);
    expect(menuRights(server(), text, "text", 10)?.canDelete).toBe(false);
    expect(menuRights(server(), text, "text", 11)?.canEdit).toBe(false);
    expect(canManageMembers(server(), text, "voice", 1)).toBe(true);
    expect(canManageMembers(server(), { ...text, private: { 3: [] } }, "voice", 3)).toBe(false);
  });
});

describe("what channel moderators do to users", () => {
  it("mute, move, disconnect and time out in their channel", () => {
    const m = userModeration(server(), text, user(2, null, 1));
    expect(m).toMatchObject({ kick: false, ban: false, timeout: false, mute: true, disconnect: true, deleteMessages: false });
    expect(m?.channelTimeout).toBe(1);
    // The channel we look at comes first.
    expect(userModeration(server(), text, user(2, null, 1), 4)?.channelTimeout).toBe(4);
    expect(userModeration(server(), text, user(2, null, 3))).toBeNull();
  });

  it("never act on staff or on the channel's other moderators", () => {
    expect(userModeration(server(), text, user(4, 7, 1))).toBeNull(); // a server moderator
    expect(userModeration(server(), text, user(3, 6, 2))).toBeNull(); // channel 2's moderator
    // Staff act on channel moderators.
    const staff = server(MOD | Perm.MakeChannel | Perm.Write, { 1: MOD, 2: MOD });
    expect(userModeration(staff, text, user(3, 6, 2))).toMatchObject({ mute: true, channelTimeout: 2 });
  });

  it("send channel time-outs to the Kallisti server", () => {
    expect(moderationRequest(4, { kind: "channelTimeout", channel: 1, seconds: 300 })).toEqual({ manage: { channelTimeout: 4, channel: 1, seconds: 300 } });
    expect(JSON.parse(manageRequest({ channelTimeout: 4, channel: 1, seconds: 300 }))).toEqual({ type: "channel-time-out", session: 4, voice: 1, seconds: 300 });
  });
});

describe("channel time-outs", () => {
  it("come with the time-outs and hold in the channel and its subchannels", () => {
    const ready = parseServerEvent('{"type":"ready","channels":[],"voiceChats":true,"channelTimeouts":[{"user":5,"channel":1,"until":5000}]}');
    const s = reduceText(initialTextState, ready!);
    expect(s.channelTimeouts).toEqual([{ user: 5, channel: 1, until: 5000 }]);
    const changed = parseServerEvent('{"type":"timeouts","timeouts":[],"channelTimeouts":[{"session":2,"channel":3,"until":6000}]}');
    const t = reduceText(s, changed!).channelTimeouts;
    expect(t).toEqual([{ session: 2, channel: 3, until: 6000 }]);
    // Older servers send none.
    expect(reduceText(s, parseServerEvent('{"type":"timeouts","timeouts":[]}')!).channelTimeouts).toEqual([]);
    expect(channelTimedOutUntil(server(), s.channelTimeouts, user(1, 5), 2)).toBe(5000);
    expect(channelTimedOutUntil(server(), s.channelTimeouts, user(1, 5), 3)).toBeNull();
    expect(channelTimedOutUntil(server(), t, user(2, null), 3)).toBe(6000);
    expect(channelTimedOutUntil(server(), t, user(2, null), null)).toBeNull();
  });
});
