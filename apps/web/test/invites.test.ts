import { describe, expect, it } from "vitest";
import { inviteExpiry, inviteLink, invitePlace, inviteUses, parseInviteLink } from "../src/lib/invites";
import { initialTextState, manageRequest, parseServerEvent, reduceText } from "../src/lib/textChannels";

const token = "AbC-_123456789012345678901234567";

describe("invite links", () => {
  it("make a link on the server", () => {
    expect(inviteLink("https://chat.example", token)).toBe(`https://chat.example/invite/${token}`);
  });

  it("read a link back, only for this server", () => {
    expect(parseInviteLink(`https://chat.example/invite/${token}`, "https://chat.example")).toBe(token);
    expect(parseInviteLink(`/invite/${token}`, "https://chat.example")).toBe(token);
    expect(parseInviteLink(`https://other.example/invite/${token}`, "https://chat.example")).toBeNull();
    expect(parseInviteLink("https://chat.example/invite/short", "https://chat.example")).toBeNull();
    expect(parseInviteLink(`https://chat.example/invite/${token}!`, "https://chat.example")).toBeNull();
    expect(parseInviteLink("https://chat.example/", "https://chat.example")).toBeNull();
  });
});

describe("inviteUses", () => {
  it("counts uses, with the limit if there is one", () => {
    expect(inviteUses({ uses: 0 })).toBe("0 uses");
    expect(inviteUses({ uses: 1 })).toBe("1 use");
    expect(inviteUses({ uses: 2, maxUses: 5 })).toBe("2 of 5 uses");
    expect(inviteUses({ uses: 0, maxUses: 1 })).toBe("0 of 1 use");
  });
});

describe("inviteExpiry", () => {
  const now = 1_000_000_000;
  const min = 60_000;
  it("says when a link ends", () => {
    expect(inviteExpiry(undefined, now)).toBe("never ends");
    expect(inviteExpiry(now + 7 * 24 * 60 * min, now)).toBe("ends in 7 days");
    expect(inviteExpiry(now + 25 * 60 * min, now)).toBe("ends in 1 day");
    expect(inviteExpiry(now + 5 * 60 * min + 1, now)).toBe("ends in 5 hours");
    expect(inviteExpiry(now + 12 * min, now)).toBe("ends in 12 minutes");
    expect(inviteExpiry(now + 10_000, now)).toBe("ends in under a minute");
    expect(inviteExpiry(now - 1, now)).toBe("ended");
  });
});

describe("invitePlace", () => {
  it("opens the text channel, or the voice channel", () => {
    expect(invitePlace({ voice: 11, channel: 2 })).toEqual({ kind: "text", id: 2 });
    expect(invitePlace({ voice: 4 })).toEqual({ kind: "voice", id: 4 });
  });
});

describe("invite events", () => {
  it("keep a channel's links", () => {
    const ev = parseServerEvent(
      JSON.stringify({ type: "invites", voice: 11, invites: [{ id: 1, creator: "mod", created: 5, expires: 9, maxUses: 5, uses: 2 }, { id: 2, creator: "mod", created: 6, uses: 0 }] }),
    );
    const state = reduceText(initialTextState, ev!);
    expect(state.invites[11]).toEqual([
      { id: 1, creator: "mod", created: 5, expires: 9, maxUses: 5, uses: 2 },
      { id: 2, creator: "mod", created: 6, uses: 0 },
    ]);
    expect(parseServerEvent(JSON.stringify({ type: "invites", voice: 11, invites: [{ id: 1 }] }))).toBeNull();
  });
  it("read a new link and a joined channel", () => {
    expect(parseServerEvent(JSON.stringify({ type: "invite-created", voice: 11, token }))).toEqual({ type: "invite-created", voice: 11, token });
    expect(parseServerEvent(JSON.stringify({ type: "invite-joined", voice: 11, channel: 2 }))).toEqual({ type: "invite-joined", voice: 11, channel: 2 });
    expect(parseServerEvent(JSON.stringify({ type: "invite-joined", voice: 4 }))).toEqual({ type: "invite-joined", voice: 4 });
    expect(parseServerEvent(JSON.stringify({ type: "invite-joined", voice: "x" }))).toBeNull();
  });
  it("are asked for", () => {
    expect(JSON.parse(manageRequest({ invites: 11 }))).toEqual({ type: "invites", voice: 11 });
    expect(JSON.parse(manageRequest({ createInvite: 11, seconds: 86400, uses: 5 }))).toEqual({ type: "create-invite", voice: 11, seconds: 86400, uses: 5 });
    expect(JSON.parse(manageRequest({ deleteInvite: 3 }))).toEqual({ type: "delete-invite", invite: 3 });
    expect(JSON.parse(manageRequest({ redeemInvite: token }))).toEqual({ type: "redeem-invite", token });
  });
});
