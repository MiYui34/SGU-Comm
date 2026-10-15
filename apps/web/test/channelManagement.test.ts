import { describe, expect, it } from "vitest";
import { initialState, Perm, type Channel, type ServerState } from "@kallisti/protocol";
import { menuRights } from "../src/lib/channelManagement";
import { initialTextState, type TextState } from "../src/lib/textChannels";

const ch = (id: number, parent: number | null, position = 0): Channel => ({
  id, parent, name: `c${id}`, description: "", descriptionHash: null, position, temporary: false,
});
const server = (perms: Record<number, number>, root: number | null = null): ServerState => ({
  ...initialState,
  channels: Object.fromEntries([ch(0, null), ch(1, 0, 0), ch(2, 0, 1), ch(3, 0, 2)].map((c) => [c.id, c])),
  channelPermissions: perms,
  rootPermissions: root,
});
const text: TextState = {
  ...initialTextState,
  voiceChats: true,
  channels: [{ id: 10, name: "general" }, { id: 11, name: "random" }],
};
const W = Perm.Write;

describe("menuRights", () => {
  it("lets admins and moderators change text channels; others get a menu without changes (to mute)", () => {
    expect(menuRights(server({}, Perm.MakeChannel), text, "text", 10)).toEqual({ canEdit: true, canDelete: true, canMoveUp: false, canMoveDown: true });
    const none = { canEdit: false, canDelete: false, canMoveUp: false, canMoveDown: false };
    expect(menuRights(server({}, 0), text, "text", 10)).toEqual(none);
    expect(menuRights(server({}, Perm.MakeChannel), { ...text, voiceChats: false }, "text", 10)).toEqual(none);
    expect(menuRights(server({}, Perm.MakeChannel), text, "text", 999)).toBeNull();
  });

  it("moves a voice channel only with Write on all its siblings", () => {
    const partly = menuRights(server({ 1: W, 2: W }), text, "voice", 2);
    expect(partly).toEqual({ canEdit: true, canDelete: true, canMoveUp: false, canMoveDown: false });
    const all = menuRights(server({ 1: W, 2: W, 3: W }), text, "voice", 2);
    expect(all).toEqual({ canEdit: true, canDelete: true, canMoveUp: true, canMoveDown: true });
  });

  it("never deletes or moves root", () => {
    expect(menuRights(server({ 0: W }), text, "voice", 0)).toEqual({ canEdit: true, canDelete: false, canMoveUp: false, canMoveDown: false });
  });

  it("has nothing for a channel that is gone", () => {
    expect(menuRights(server({}), text, "voice", 99)).toBeNull();
    expect(menuRights(server({}, Perm.MakeChannel), text, "text", 99)).toBeNull();
  });
});

describe("menuRights with text channel mirrors", () => {
  it("ignores the mirrors' parent among the siblings", () => {
    const base = server({ 1: W, 2: W, 3: W });
    const mirror = (id: number, parent: number, name: string, position: number): Channel => ({
      ...ch(id, parent, position),
      name,
    });
    const s: ServerState = {
      ...base,
      channels: { ...base.channels, 5: mirror(5, 0, "Text channels", 1000), 6: mirror(6, 5, "#x", 0) },
    };
    const t: TextState = { ...text, channels: [...text.channels, { id: 12, name: "x", mumble: 6 }] };
    expect(menuRights(s, t, "voice", 3)?.canMoveDown).toBe(false);
    expect(menuRights(s, t, "voice", 3)?.canMoveUp).toBe(true);
  });
});
