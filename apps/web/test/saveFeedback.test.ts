import { describe, expect, it } from "vitest";
import { answer, busyLabel, confirmsManage, fade, idle, pending, settle } from "../src/lib/saveFeedback";
import type { ServerEvent } from "../src/lib/textChannels";

describe("save states", () => {
  it("goes from pending to saved, or to an error", () => {
    expect(settle(pending)).toEqual({ phase: "saved" });
    expect(settle(pending, "no")).toEqual({ phase: "error", message: "no" });
  });
  it("takes an answer only while pending", () => {
    expect(settle(idle)).toBe(idle);
    const err = settle(pending, "no");
    expect(settle(err)).toBe(err);
  });
  it("fades saved back to idle, but keeps an error", () => {
    expect(fade(settle(pending))).toBe(idle);
    const err = settle(pending, "no");
    expect(fade(err)).toBe(err);
  });
  it("labels the button while pending", () => {
    expect(busyLabel(pending, "Save")).toBe("Saving…");
    expect(busyLabel(idle, "Save")).toBe("Save");
    expect(busyLabel(pending, "Delete", "Deleting…")).toBe("Deleting…");
  });
});

describe("answer", () => {
  type W = { via: "text" | "mumble"; id: number; ok?: boolean };
  const ws: W[] = [
    { via: "mumble", id: 1, ok: true },
    { via: "text", id: 2 },
    { via: "text", id: 3, ok: true },
  ];
  it("gives a refusal to the oldest waiter of its kind", () => {
    const r = answer(ws, "text", true, () => false);
    expect(r.refused?.id).toBe(2);
    expect(r.confirmed).toEqual([]);
    expect(r.rest.map((w) => w.id)).toEqual([1, 3]);
  });
  it("confirms the oldest waiter of its kind the answer matches", () => {
    const r = answer([...ws, { via: "text", id: 4, ok: true }], "text", false, (w) => !!w.ok);
    expect(r.refused).toBeUndefined();
    expect(r.confirmed.map((w) => w.id)).toEqual([3]);
    expect(r.rest.map((w) => w.id)).toEqual([1, 2, 4]);
  });
  it("can refuse one and confirm the others whose Mumble state holds", () => {
    const more: W[] = [...ws, { via: "mumble", id: 4, ok: true }, { via: "mumble", id: 5, ok: true }];
    const r = answer(more, "mumble", true, (w) => !!w.ok);
    expect(r.refused?.id).toBe(1);
    expect(r.confirmed.map((w) => w.id)).toEqual([4, 5]);
    expect(r.rest.map((w) => w.id)).toEqual([2, 3]);
  });
  it("leaves the queue as it is without a waiter", () => {
    const r = answer(ws, "text", false, () => false);
    expect(r.rest).toEqual(ws);
  });
});

describe("confirmsManage", () => {
  const ev = (e: ServerEvent) => e;
  it("waits for the setting's new value", () => {
    const c = confirmsManage({ guestsReadOnly: true })!;
    expect(c(ev({ type: "guests-read-only", on: true }))).toBe(true);
    expect(c(ev({ type: "guests-read-only", on: false }))).toBe(false);
    expect(confirmsManage({ captcha: "hard" })!(ev({ type: "captcha", level: "hard" }))).toBe(true);
    expect(confirmsManage({ captcha: "hard" })!(ev({ type: "captcha", level: "off" }))).toBe(false);
  });
  it("waits for the saved message rules", () => {
    const rules = { newMinutes: 0, words: [], domains: [], replacements: [] };
    expect(confirmsManage({ setMessageRules: rules })!(ev({ type: "message-rules", rules }))).toBe(true);
  });
  it("waits for the changed channel, not any channel list", () => {
    const channels = (...cs: { id: number; name: string; topic?: string; slowMode?: number; category?: number }[]) => ev({ type: "channels", channels: cs });
    const rename = confirmsManage({ rename: 1, name: "News Room" })!;
    expect(rename(channels({ id: 1, name: "general" }))).toBe(false); // the privacy change's list, before the rename
    expect(rename(channels({ id: 1, name: "news-room" }))).toBe(true); // as the server writes it
    expect(confirmsManage({ slowMode: 1, seconds: 5 })!(channels({ id: 1, name: "a" }))).toBe(false);
    expect(confirmsManage({ slowMode: 1, seconds: 5 })!(channels({ id: 1, name: "a", slowMode: 5 }))).toBe(true);
    expect(confirmsManage({ topic: 1, text: "" })!(channels({ id: 1, name: "a" }))).toBe(true);
    expect(confirmsManage({ setCategory: 1, category: 0 })!(channels({ id: 1, name: "a", category: 2 }))).toBe(false);
    expect(confirmsManage({ create: "Hi There" })!(channels({ id: 4, name: "hi-there" }))).toBe(true);
    expect(confirmsManage({ remove: 1 })!(channels({ id: 1, name: "a" }))).toBe(false);
    expect(confirmsManage({ order: [2, 1] })!(channels({ id: 1, name: "a" }, { id: 2, name: "b" }))).toBe(false);
    expect(confirmsManage({ order: [2, 1] })!(channels({ id: 3, name: "c" }, { id: 2, name: "b" }, { id: 1, name: "a" }))).toBe(true);
    const category = (name: string) => ev({ type: "channels", channels: [], categories: [{ id: 3, name }] });
    expect(confirmsManage({ renameCategory: 3, name: " Fun  Stuff " })!(category("Games"))).toBe(false);
    expect(confirmsManage({ renameCategory: 3, name: " Fun  Stuff " })!(category("Fun Stuff"))).toBe(true);
    expect(confirmsManage({ removeCategory: 3 })!(category("Games"))).toBe(false);
  });
  it("waits for the channel's new privacy", () => {
    const priv = confirmsManage({ setPrivate: 1, on: true, users: [] })!;
    expect(priv(ev({ type: "private", channels: [] }))).toBe(false);
    expect(priv(ev({ type: "private", channels: [{ channel: 1 }] }))).toBe(true);
    expect(confirmsManage({ setPrivate: 1, on: false, users: [] })!(ev({ type: "private", channels: [] }))).toBe(true);
  });
  it("waits for the changed lists", () => {
    expect(confirmsManage({ channelModerator: 1, channel: 2, on: true })!(ev({ type: "roles", moderators: [], admins: [] }))).toBe(true);
    expect(confirmsManage({ deleteInvite: 1 })!(ev({ type: "invites", voice: 2, invites: [] }))).toBe(true);
    expect(confirmsManage({ rename: 1, name: "a" })!(ev({ type: "private", channels: [] }))).toBe(false);
  });
  it("has nothing to wait for where the server sends no answer of its own", () => {
    expect(confirmsManage({ ban: 1, reason: "", seconds: 60 })).toBeNull();
    expect(confirmsManage({ deleteMessages: { user: 1 }, seconds: 60 })).toBeNull();
  });
});
