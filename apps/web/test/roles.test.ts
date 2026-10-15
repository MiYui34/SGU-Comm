import { describe, expect, it } from "vitest";
import { initialState, Perm, type ServerState, type User } from "@kallisti/protocol";
import { moderatorAction, roleOf } from "../src/lib/roles";
import { initialTextState, type TextState } from "../src/lib/textChannels";

const text: TextState = { ...initialTextState, voiceChats: true, roles: { admins: [3], moderators: [5] } };
const admin: ServerState = { ...initialState, rootPermissions: Perm.Write | Perm.MakeChannel };
const user = (userId: number | null) => ({ session: 9, name: "x", userId }) as User;

describe("moderatorAction", () => {
  it("lets admins make and unmake moderators", () => {
    expect(moderatorAction(admin, text, user(6))).toBe("make");
    expect(moderatorAction(admin, text, user(5))).toBe("remove");
  });

  it("offers nothing for guests, admins, or without the right", () => {
    expect(moderatorAction(admin, text, user(null))).toBeNull();
    expect(moderatorAction(admin, text, user(3))).toBeNull();
    expect(moderatorAction(admin, text, user(0))).toBeNull();
    expect(moderatorAction({ ...admin, rootPermissions: Perm.MakeChannel }, text, user(6))).toBeNull();
    expect(moderatorAction(admin, { ...text, voiceChats: false }, user(6))).toBeNull();
  });
});

describe("roleOf", () => {
  it("name a user's role", () => {
    const roles = { admins: [3], moderators: [5] };
    expect(roleOf(roles, 0)).toBe("admin"); // SuperUser
    expect(roleOf(roles, 3)).toBe("admin");
    expect(roleOf(roles, 5)).toBe("moderator");
    expect(roleOf(roles, 6)).toBeNull();
    expect(roleOf(roles, null)).toBeNull(); // guests
  });
});
