import { describe, expect, it } from "vitest";
import { initialState, Perm, type ServerState } from "@kallisti/protocol";
import { moderationRights } from "../src/lib/moderation";

const root = (perms: number): ServerState => ({ ...initialState, rootPermissions: perms });

describe("moderationRights", () => {
  it("follows the root permissions", () => {
    expect(moderationRights(root(0))).toEqual({ any: false, bans: false, editBans: false, users: false });
    expect(moderationRights(root(Perm.Ban | Perm.Register))).toEqual({ any: true, bans: true, editBans: false, users: true });
    expect(moderationRights(root(Perm.Ban | Perm.Register | Perm.Write))).toEqual({
      any: true,
      bans: true,
      editBans: true,
      users: true,
    });
  });
});
