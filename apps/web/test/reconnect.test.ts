import { describe, expect, it } from "vitest";
import { proto } from "@kallisti/protocol";
import { busyName, loggedOut, reconnectDelay, rejoinTarget, shouldReconnect } from "../src/lib/reconnect";
import type { Place } from "../src/lib/place";

const lost = { status: "synced" as const, closed: true, closeReason: "", wasSynced: true };

describe("shouldReconnect", () => {
  it("comes back after an unexpected close", () => {
    expect(shouldReconnect(lost)).toBe(true);
    expect(shouldReconnect({ ...lost, closeReason: "server shutting down" })).toBe(true); // a restart
  });

  it("not while open, before it was ever in, or after a refusal", () => {
    expect(shouldReconnect({ ...lost, closed: false })).toBe(false);
    expect(shouldReconnect({ ...lost, wasSynced: false })).toBe(false);
    expect(shouldReconnect({ ...lost, status: "kicked" })).toBe(false);
    expect(shouldReconnect({ ...lost, status: "rejected" })).toBe(false);
    expect(shouldReconnect({ ...lost, closeReason: "certificate-changed ab" })).toBe(false);
  });

  it("not after the server ended the session (logout elsewhere)", () => {
    expect(shouldReconnect({ ...lost, closeReason: "logged out" })).toBe(false);
  });
});

describe("loggedOut", () => {
  it("is the server's close after a logout or password change", () => {
    expect(loggedOut("logged out")).toBe(true);
    expect(loggedOut("no authentication")).toBe(false);
    expect(loggedOut("")).toBe(false);
  });
});

describe("reconnectDelay", () => {
  it("waits longer each time, up to 30 seconds", () => {
    expect([0, 1, 2, 3, 4, 5, 9].map(reconnectDelay)).toEqual([1000, 2000, 5000, 10000, 20000, 30000, 30000]);
  });
});

describe("busyName", () => {
  it("is a refusal because the old session still holds the name", () => {
    expect(busyName({ status: "rejected", rejectType: proto.Reject_RejectType.UsernameInUse })).toBe(true);
    expect(busyName({ status: "rejected", rejectType: proto.Reject_RejectType.WrongUserPW })).toBe(false);
    expect(busyName({ status: "synced", rejectType: null })).toBe(false);
  });
});

describe("rejoinTarget", () => {
  const back: Place = { session: 3, view: { kind: "voice" }, voice: 5, thread: null, selfMute: true, selfDeaf: false };
  const exists = (id: number) => id !== 9;

  it("the voice channel the user was in", () => {
    expect(rejoinTarget(back, 0, exists, null)).toBe(5);
  });

  it("none when not in voice before, already there, or the channel is gone", () => {
    expect(rejoinTarget(null, 0, exists, null)).toBe(null);
    expect(rejoinTarget({ ...back, voice: null }, 0, exists, null)).toBe(null);
    expect(rejoinTarget(back, 5, exists, null)).toBe(null);
    expect(rejoinTarget({ ...back, voice: 9 }, 0, exists, null)).toBe(null);
  });

  it("takes voice back from our old session, which the server has not let go yet", () => {
    expect(rejoinTarget(back, 0, exists, 3)).toBe(5);
  });

  it("not from another device of the account that went into voice meanwhile", () => {
    expect(rejoinTarget(back, 0, exists, 4)).toBe(null);
    expect(rejoinTarget({ ...back, session: null }, 0, exists, 4)).toBe(null);
  });
});
