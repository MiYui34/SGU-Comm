import { describe, expect, it } from "vitest";
import { initialState, Perm, type ServerState, type User } from "@kallisti/protocol";
import { loginAction, mayChangeGuests, mayWrite } from "../src/lib/guests";
import { initialTextState, manageRequest, parseServerEvent, reduceText, type TextState } from "../src/lib/textChannels";

const on: TextState = { ...initialTextState, voiceChats: true, guestsReadOnly: true, roles: { admins: [3], moderators: [5] } };
// I am session 1, with this user id (null: a guest).
const as = (userId: number | null, rootPermissions = 0): ServerState => ({
  ...initialState,
  rootPermissions,
  session: 1,
  users: { 1: { session: 1, name: "me", userId } as User },
});

describe("mayWrite", () => {
  it("lets everyone write while the setting is off", () => {
    expect(mayWrite(as(null), { ...on, guestsReadOnly: false }, false)).toBe(true);
    expect(mayWrite(as(7), { ...on, guestsReadOnly: false }, true)).toBe(true);
  });

  it("keeps guests and guest accounts from writing while it is on", () => {
    expect(mayWrite(as(null), on, false)).toBe(false);
    expect(mayWrite(as(7), on, true)).toBe(false);
    expect(mayWrite(as(null, Perm.Write), on, false)).toBe(false); // staff are registered
  });

  it("lets registered users and staff write", () => {
    expect(mayWrite(as(7), on, false)).toBe(true);
    expect(mayWrite(as(5), on, true)).toBe(true); // a moderator's guest account
    expect(mayWrite(as(8, Perm.Write), on, true)).toBe(true); // an admin by ACL
  });
});

describe("the setting", () => {
  it("comes with ready and changes later", () => {
    const s = reduceText(initialTextState, parseServerEvent('{"type":"ready","channels":[],"guestsReadOnly":true}')!);
    expect(s.guestsReadOnly).toBe(true);
    expect(reduceText(s, parseServerEvent('{"type":"guests-read-only","on":false}')!).guestsReadOnly).toBe(false);
    expect(reduceText(s, parseServerEvent('{"type":"ready","channels":[]}')!).guestsReadOnly).toBe(false);
    expect(parseServerEvent('{"type":"guests-read-only","on":"yes"}')).toBeNull();
  });

  it("is changed by admins", () => {
    expect(manageRequest({ guestsReadOnly: true })).toBe('{"type":"set-guests-read-only","on":true}');
    expect(mayChangeGuests(as(3, Perm.Write), on)).toBe(true);
    expect(mayChangeGuests(as(5), on)).toBe(false);
    expect(mayChangeGuests(as(3, Perm.Write), { ...on, voiceChats: false })).toBe(false);
  });
});

describe("loginAction", () => {
  it("offers a password to guest accounts and the login to others", () => {
    expect(loginAction(true)).toBe("Set a password");
    expect(loginAction(false)).toBe("Log in");
  });
});
