import { describe, expect, it } from "vitest";
import { Perm } from "@kallisti/protocol";
import { goLiveEntries, isLive, streamBlock, type GoLiveState } from "../src/lib/stream/goLive";

const idle: GoLiveState = {
  sharing: { screen: false, camera: false },
  obsLive: false,
  sources: { screen: true, camera: true },
  whip: true,
  password: true,
  blocked: null,
};
const labels = (s: Partial<GoLiveState>) => goLiveEntries({ ...idle, ...s }).map((e) => e.label);

describe("goLiveEntries", () => {
  it("offers the screen, the camera and OBS", () => {
    expect(goLiveEntries(idle)).toEqual([
      { action: "screen", label: "Share your screen" },
      { action: "camera", label: "Turn on camera" },
      { action: "obs", label: "Stream from OBS…" },
    ]);
  });

  it("offers to stop what runs", () => {
    expect(labels({ sharing: { screen: true, camera: true } })).toEqual(["Stop sharing", "Turn off camera", "Stream from OBS…"]);
  });

  it("leaves out what the browser cannot share (phones: no screen)", () => {
    expect(labels({ sources: { screen: false, camera: true } })).toEqual(["Turn on camera", "Stream from OBS…"]);
    expect(labels({ sources: { screen: false, camera: false } })).toEqual(["Stream from OBS…"]);
  });

  it("offers OBS only where the server has WHIP, to accounts with a password", () => {
    expect(labels({ whip: false })).toEqual(["Share your screen", "Turn on camera"]);
    expect(labels({ password: false })).toEqual(["Share your screen", "Turn on camera"]);
  });

  it("disables starting where we may not stream, but not stopping", () => {
    const why = "You may not speak in this channel";
    expect(goLiveEntries({ ...idle, blocked: why, sharing: { screen: false, camera: true } })).toEqual([
      { action: "screen", label: "Share your screen", off: why },
      { action: "camera", label: "Turn off camera" },
      { action: "obs", label: "Stream from OBS…" },
    ]);
  });

  it("disables sharing the screen while OBS streams", () => {
    expect(goLiveEntries({ ...idle, obsLive: true })[0]).toEqual({
      action: "screen",
      label: "Share your screen",
      off: "Stop the OBS stream first",
    });
  });
});

describe("isLive", () => {
  it("is on while the screen, the camera or OBS streams", () => {
    expect(isLive(idle)).toBe(false);
    expect(isLive({ ...idle, sharing: { screen: true, camera: false } })).toBe(true);
    expect(isLive({ ...idle, sharing: { screen: false, camera: true } })).toBe(true);
    expect(isLive({ ...idle, obsLive: true })).toBe(true);
  });
});

describe("streamBlock", () => {
  const me = { mute: false, suppress: false };
  it("allows streaming while the permissions are unknown", () => {
    expect(streamBlock(me, undefined)).toBeNull();
    expect(streamBlock(me, Perm.Speak | Perm.Enter)).toBeNull();
  });

  it("says why we may not stream", () => {
    expect(streamBlock(me, Perm.Enter)).toBe("You may not speak in this channel");
    expect(streamBlock({ ...me, suppress: true }, undefined)).toBe("You may not speak in this channel");
    expect(streamBlock({ ...me, mute: true }, Perm.Speak)).toBe("A moderator muted you");
  });
});
