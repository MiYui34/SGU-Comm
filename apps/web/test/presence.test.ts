import { describe, expect, it } from "vitest";
import {
  AWAY_AFTER_MS,
  awayRequest,
  clearAfterUntil,
  customOf,
  effectivePresence,
  isAway,
  presenceOf,
  setStatusRequest,
} from "../src/lib/presence";
import { initialTextState, parseServerEvent, reduceText } from "../src/lib/textChannels";
import { shouldNotify } from "../src/lib/notify";

describe("effectivePresence", () => {
  it("is dnd if chosen, else idle if chosen or away, else online", () => {
    expect(effectivePresence("online", false)).toBe("online");
    expect(effectivePresence("online", true)).toBe("idle");
    expect(effectivePresence("idle", false)).toBe("idle");
    expect(effectivePresence("dnd", true)).toBe("dnd");
  });
});

describe("isAway", () => {
  const now = 100 * AWAY_AFTER_MS;
  it("after 10 minutes without input or hidden", () => {
    expect(isAway({ lastInput: now - AWAY_AFTER_MS + 1, hiddenSince: null, now })).toBe(false);
    expect(isAway({ lastInput: now - AWAY_AFTER_MS, hiddenSince: null, now })).toBe(true);
    expect(isAway({ lastInput: now, hiddenSince: now - AWAY_AFTER_MS, now })).toBe(true);
    expect(isAway({ lastInput: now - 1000, hiddenSince: now - 1000, now })).toBe(false);
  });
});

describe("custom status", () => {
  it("shows until its end", () => {
    expect(customOf({ status: "online" }, 5)).toBeNull();
    expect(customOf({ status: "dnd", emoji: "🍕", text: "lunch", until: 10 }, 5)).toEqual({ emoji: "🍕", text: "lunch" });
    expect(customOf({ status: "dnd", text: "lunch", until: 10 }, 10)).toBeNull();
    expect(customOf({ status: "idle", emoji: "🍕" }, 5)).toEqual({ emoji: "🍕", text: "" });
    expect(customOf(undefined, 5)).toBeNull();
  });

  it("clear after: a time, the end of today, or never", () => {
    const now = new Date(2026, 9, 17, 15, 30).getTime();
    expect(clearAfterUntil("30m", now)).toBe(now + 30 * 60_000);
    expect(clearAfterUntil("1h", now)).toBe(now + 3_600_000);
    expect(clearAfterUntil("4h", now)).toBe(now + 4 * 3_600_000);
    expect(clearAfterUntil("today", now)).toBe(new Date(2026, 9, 18).getTime());
    expect(clearAfterUntil("never", now)).toBeUndefined();
  });

  it("requests", () => {
    expect(JSON.parse(setStatusRequest("dnd", { emoji: "🍕", text: "lunch", until: 9 }))).toEqual({
      type: "set-status",
      status: "dnd",
      emoji: "🍕",
      text: "lunch",
      until: 9,
    });
    expect(JSON.parse(setStatusRequest("idle"))).toEqual({ type: "set-status", status: "idle" });
    expect(JSON.parse(awayRequest(true))).toEqual({ type: "away", on: true });
  });
});

describe("status events", () => {
  it("come with ready and as events; plain online drops out", () => {
    const ready = parseServerEvent(
      JSON.stringify({
        type: "ready",
        channels: [],
        statuses: [{ session: 3, status: "dnd", text: "busy" }, { session: 4, status: "bogus" }],
        ownStatus: { status: "idle", emoji: "🍕", until: 7 },
      }),
    );
    let state = reduceText(initialTextState, ready!);
    expect(state.statuses).toEqual({ 3: { status: "dnd", text: "busy" } });
    expect(state.ownStatus).toEqual({ status: "idle", emoji: "🍕", until: 7 });
    expect(presenceOf(state.statuses, 3)).toBe("dnd");
    expect(presenceOf(state.statuses, 9)).toBe("online");

    state = reduceText(state, parseServerEvent('{"type":"status","session":5,"status":"idle"}')!);
    expect(state.statuses[5]).toEqual({ status: "idle" });
    state = reduceText(state, parseServerEvent('{"type":"status","session":3,"status":"online"}')!);
    expect(state.statuses[3]).toBeUndefined();
    state = reduceText(state, parseServerEvent('{"type":"own-status","status":"dnd","emoji":"","text":"","until":0}')!);
    expect(state.ownStatus).toEqual({ status: "dnd" });
    expect(parseServerEvent('{"type":"status","session":"x","status":"idle"}')).toBeNull();
    expect(parseServerEvent('{"type":"own-status","status":"invisible"}')).toBeNull();
  });
});

describe("do not disturb", () => {
  it("no desktop notifications", () => {
    const base = { enabled: true, permission: "granted" as NotificationPermission, visible: false, deaf: false, open: false };
    expect(shouldNotify({ ...base, dnd: true })).toBe(false);
  });
});
