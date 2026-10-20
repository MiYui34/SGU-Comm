import { describe, expect, it } from "vitest";
import { emptyUser, initialState, Perm, type Channel, type ServerState } from "@kallisti/protocol";
import { collapsedIn, parseCollapsed, shownCollapsed, textGroups, textOrder, toggleCollapsed } from "../src/lib/categories";
import { canCreateText, managesCategory, menuRights, moderatorsChannel } from "../src/lib/channelManagement";
import { mirrorIds } from "../src/lib/mirrors";
import { initialTextState, manageRequest, parseServerEvent, reduceText, type TextState } from "../src/lib/textChannels";

const ch = (id: number, parent: number | null, name = `c${id}`): Channel => ({
  id, parent, name, description: "", descriptionHash: null, position: 0, temporary: false,
});
// The mirror tree: "Text channels" (5) with #general (6) and Games (7),
// which holds #minecraft (8) and #valorant (9); Music (4) is empty. I am
// session 1 (user 5); user 5 moderates Games.
const server = (rootPerms = 0): ServerState => ({
  ...initialState,
  session: 1,
  rootPermissions: rootPerms,
  channels: Object.fromEntries(
    [ch(0, null), ch(1, 0), ch(5, 0, "Text channels"), ch(6, 5), ch(7, 5, "Games"), ch(8, 7), ch(9, 7), ch(4, 5, "Music")].map((c) => [c.id, c]),
  ),
  users: { 1: { ...emptyUser(1), name: "me", userId: 5 } },
});
const text: TextState = {
  ...initialTextState,
  voiceChats: true,
  channels: [
    { id: 11, name: "minecraft", mumble: 8, category: 1 },
    { id: 10, name: "general", mumble: 6 },
    { id: 12, name: "valorant", mumble: 9, category: 1 },
    { id: 13, name: "lost", category: 99 },
  ],
  categories: [{ id: 1, name: "Games", mumble: 7 }, { id: 2, name: "Music", mumble: 4 }],
  roles: { admins: [], moderators: [], channelMods: { 7: [5] } },
};
const staff = server(Perm.MakeChannel | Perm.Write);
const ids = (cs: readonly { id: number }[]) => cs.map((c) => c.id);

describe("text categories", () => {
  it("group the channels: those without a (known) category first, then each category in order", () => {
    const groups = textGroups(text.channels, text.categories);
    expect(groups.map((g) => [g.category?.name ?? null, ids(g.channels)])).toEqual([
      [null, [10, 13]],
      ["Games", [11, 12]],
      ["Music", []],
    ]);
    expect(ids(textOrder(text.channels, text.categories))).toEqual([10, 13, 11, 12]);
  });

  it("collapsed, still show the open channel and unread ones", () => {
    const games = text.channels.filter((c) => c.category === 1);
    expect(ids(shownCollapsed(games, 12, new Set()))).toEqual([12]);
    expect(ids(shownCollapsed(games, null, new Set([11])))).toEqual([11]);
  });

  it("keep their collapsed state per server on this device", () => {
    const one = toggleCollapsed({}, "https://a", 1);
    expect(collapsedIn(one, "https://a")).toEqual(new Set([1]));
    expect(collapsedIn(one, "https://b").size).toBe(0);
    expect(toggleCollapsed(one, "https://a", 1)).toEqual({});
    expect(parseCollapsed(JSON.stringify(one))).toEqual(one);
    expect(parseCollapsed('{"https://a":[1,"x",-2],"b":3}')).toEqual({ "https://a": [1] });
    expect(parseCollapsed("nonsense")).toEqual({});
  });

  it("come with ready and channels events", () => {
    const ready = parseServerEvent('{"type":"ready","channels":[{"id":1,"name":"a","category":2}],"categories":[{"id":2,"name":"Games","mumble":7}]}');
    const s = reduceText(initialTextState, ready!);
    expect(s.categories).toEqual([{ id: 2, name: "Games", mumble: 7 }]);
    expect(s.channels[0]?.category).toBe(2);
    expect(reduceText(s, parseServerEvent('{"type":"channels","channels":[]}')!).categories).toEqual([]);
    expect(parseServerEvent('{"type":"channels","channels":[],"categories":[{"id":"x"}]}')).toBeNull();
    expect(parseServerEvent('{"type":"channels","channels":[{"id":1,"name":"a","category":"x"}]}')).toBeNull();
  });

  it("are managed with these requests", () => {
    const req = (a: Parameters<typeof manageRequest>[0]) => JSON.parse(manageRequest(a));
    expect(req({ createCategory: "Games" })).toEqual({ type: "create-category", name: "Games" });
    expect(req({ renameCategory: 1, name: "Gaming" })).toEqual({ type: "rename-category", category: 1, name: "Gaming" });
    expect(req({ removeCategory: 1 })).toEqual({ type: "delete-category", category: 1 });
    expect(req({ orderCategories: [2, 1] })).toEqual({ type: "order-categories", categories: [2, 1] });
    expect(req({ setCategory: 10, category: 0 })).toEqual({ type: "set-category", channel: 10, category: 0 });
    expect(req({ create: "minecraft", category: 1 })).toEqual({ type: "create-channel", name: "minecraft", category: 1 });
  });

  it("are hidden from the voice channels, with their parent, also when empty", () => {
    expect([...mirrorIds(server(), text.channels, text.categories)].sort()).toEqual([4, 5, 6, 7, 8, 9]);
    const onlyInCategories = text.channels.filter((c) => c.category === 1);
    expect([...mirrorIds(server(), onlyInCategories)].sort()).toEqual([5, 7, 8, 9]);
  });
});

describe("who manages text categories", () => {
  it("staff manage all; a category's moderators theirs", () => {
    expect(managesCategory(staff, text, 2)).toBe(true);
    expect(managesCategory(server(), text, 1)).toBe(true);
    expect(managesCategory(server(), text, 2)).toBe(false);
    expect(managesCategory(server(), text, undefined)).toBe(false);
    expect(canCreateText(server(), text, 1)).toBe(true);
    expect(canCreateText(server(), text, undefined)).toBe(false);
    expect(canCreateText(staff, text, undefined)).toBe(true);
    expect(canCreateText(server(), { ...text, voiceChats: false }, 1)).toBe(false);
  });

  it("staff edit, move and delete categories and appoint their moderators; others get no menu", () => {
    expect(menuRights(staff, text, "category", 1)).toEqual({ canEdit: true, canDelete: true, canMoveUp: false, canMoveDown: true });
    expect(menuRights(server(), text, "category", 1)).toBeNull();
    expect(menuRights(staff, text, "category", 99)).toBeNull();
    expect(moderatorsChannel(staff, text, "category", 1)).toBe(7);
    expect(moderatorsChannel(server(), text, "category", 1)).toBeNull();
  });

  it("a category's moderators edit, delete and move its channels, within it", () => {
    expect(menuRights(server(), text, "text", 11)).toEqual({ canEdit: true, canDelete: true, canMoveUp: false, canMoveDown: true });
    expect(menuRights(server(), text, "text", 12)).toEqual({ canEdit: true, canDelete: true, canMoveUp: true, canMoveDown: false });
    expect(menuRights(server(), text, "text", 10)).toEqual({ canEdit: false, canDelete: false, canMoveUp: false, canMoveDown: false });
    // Staff move channels within their group too.
    expect(menuRights(staff, text, "text", 10)).toEqual({ canEdit: true, canDelete: true, canMoveUp: false, canMoveDown: true });
    expect(menuRights(staff, text, "text", 13)).toEqual({ canEdit: true, canDelete: true, canMoveUp: true, canMoveDown: false });
  });
});
