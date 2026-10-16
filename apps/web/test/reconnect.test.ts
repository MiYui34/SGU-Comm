import { describe, expect, it } from "vitest";
import { proto } from "@kallisti/protocol";
import { busyName, loggedOut, reconnectDelay, shouldReconnect } from "../src/lib/reconnect";

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
