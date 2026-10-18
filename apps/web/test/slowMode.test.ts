import { describe, expect, it } from "vitest";
import { formatSeconds, lastPostAt, SLOW_MODES, slowLeft, slowModeLabel } from "../src/lib/slowMode";
import { initialTextState, manageRequest, parseServerEvent, reduceText } from "../src/lib/textChannels";

describe("slowModeLabel", () => {
  it("names each choice", () => {
    expect(SLOW_MODES.map(slowModeLabel)).toEqual(["Off", "5 s", "30 s", "1 min", "5 min", "1 h"]);
  });
});

describe("formatSeconds", () => {
  it("leaves out empty parts", () => {
    expect(formatSeconds(23)).toBe("23 s");
    expect(formatSeconds(65)).toBe("1 min 5 s");
    expect(formatSeconds(3599)).toBe("59 min 59 s");
    expect(formatSeconds(3660)).toBe("1 h 1 min");
  });
});

describe("slowLeft", () => {
  it("counts from our last post", () => {
    expect(slowLeft(30, 1_000, undefined, 8_000)).toBe(23_000);
    expect(slowLeft(30, 1_000, undefined, 40_000)).toBe(0);
    expect(slowLeft(30, undefined, undefined, 8_000)).toBe(0);
  });

  it("takes the server's refusal when it is later (after a reload we know no post)", () => {
    expect(slowLeft(30, undefined, 20_000, 8_000)).toBe(12_000);
    expect(slowLeft(30, 1_000, 2_000, 8_000)).toBe(23_000);
  });

  it("is nothing without slow mode and never more than it (clocks differ)", () => {
    expect(slowLeft(0, 1_000, 20_000, 8_000)).toBe(0);
    expect(slowLeft(5, 100_000, undefined, 8_000)).toBe(5_000);
  });
});

describe("lastPostAt", () => {
  it("is the time of our newest message", () => {
    const msgs = [
      { at: 1, me: true },
      { at: 5, me: true },
      { at: 9, me: false },
    ];
    expect(lastPostAt(msgs, (m) => m.me)).toBe(5);
    expect(lastPostAt(msgs, () => false)).toBeUndefined();
  });
});

describe("the protocol", () => {
  it("has the setting in the channel list", () => {
    const s = reduceText(initialTextState, parseServerEvent('{"type":"ready","channels":[{"id":1,"name":"general","slowMode":30}]}')!);
    expect(s.channels[0]!.slowMode).toBe(30);
    expect(parseServerEvent('{"type":"channels","channels":[{"id":1,"name":"general","slowMode":"30"}]}')).toBeNull();
  });

  it("sets it", () => {
    expect(manageRequest({ slowMode: 1, seconds: 60 })).toBe('{"type":"set-slow-mode","channel":1,"seconds":60}');
  });

  it("keeps when the server's refusal ends", () => {
    const ev = parseServerEvent('{"type":"error","message":"Slow mode: wait 23 s","channel":1,"until":5000}')!;
    const s = reduceText(initialTextState, ev);
    expect(s.lastError).toBe("Slow mode: wait 23 s");
    expect(s.slowUntil).toEqual({ 1: 5000 });
    expect(reduceText(s, parseServerEvent('{"type":"error","message":"no"}')!).slowUntil).toEqual({ 1: 5000 });
  });
});
