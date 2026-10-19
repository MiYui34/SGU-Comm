import { describe, expect, it } from "vitest";
import {
  closeDirect,
  directPrefsOf,
  parseDirectPrefs,
  quietDirects,
  reopenDirect,
  setDirectPrefs,
  togglePin,
  visibleDirects,
} from "../src/lib/directList";

const A = "https://a";
const dm = (channel: number, latest: number) => ({ channel, latest });

describe("direct chat list", () => {
  it("reads stored data and drops what is not valid", () => {
    expect(parseDirectPrefs(null)).toEqual({});
    expect(parseDirectPrefs("nope")).toEqual({});
    expect(parseDirectPrefs("[1]")).toEqual({});
    expect(parseDirectPrefs(JSON.stringify({ [A]: { closed: { 3: 10, x: 1, 4: "y", 5: -1 }, pinned: [7, "z", 7, 8] }, b: 1 }))).toEqual({
      [A]: { closed: { 3: 10 }, pinned: [7, 8] },
    });
    expect(directPrefsOf({}, A)).toEqual({ closed: {}, pinned: [] });
  });

  it("sets per server, and forgets empty ones", () => {
    const all = setDirectPrefs({}, A, { closed: { 1: 2 }, pinned: [] });
    expect(directPrefsOf(all, A)).toEqual({ closed: { 1: 2 }, pinned: [] });
    expect(directPrefsOf(all, "https://b")).toEqual({ closed: {}, pinned: [] });
    expect(setDirectPrefs(all, A, { closed: {}, pinned: [] })).toEqual({});
  });

  it("orders pinned chats first (in pin order), then the newest first", () => {
    const list = [dm(1, 5), dm(2, 9), dm(3, 7), dm(4, 1)];
    expect(visibleDirects(list, { closed: {}, pinned: [] }).map((d) => d.channel)).toEqual([2, 3, 1, 4]);
    expect(visibleDirects(list, { closed: {}, pinned: [4, 1, 99] }).map((d) => d.channel)).toEqual([4, 1, 2, 3]);
  });

  it("hides closed chats until a newer message comes", () => {
    let p = closeDirect({ closed: {}, pinned: [] }, 2, 9);
    expect(visibleDirects([dm(1, 5), dm(2, 9)], p).map((d) => d.channel)).toEqual([1]);
    expect(visibleDirects([dm(1, 5), dm(2, 10)], p).map((d) => d.channel)).toEqual([2, 1]);
    p = closeDirect(togglePin(p, 1), 1, 5);
    expect(visibleDirects([dm(1, 5), dm(2, 9)], p)).toEqual([]);
  });

  it("opens a closed chat again; an open one stays the same object", () => {
    const p = closeDirect({ closed: {}, pinned: [] }, 2, 9);
    expect(reopenDirect(p, 2)).toEqual({ closed: {}, pinned: [] });
    expect(reopenDirect(p, 3)).toBe(p);
  });

  it("pins at the end, and unpins", () => {
    let p = togglePin({ closed: {}, pinned: [] }, 3);
    p = togglePin(p, 1);
    expect(p.pinned).toEqual([3, 1]);
    expect(togglePin(p, 3).pinned).toEqual([1]);
  });

  it("drops the counts of muted direct chats only", () => {
    const counts = { 1: 2, 2: 3, 3: 1 };
    expect(quietDirects(counts, { 1: {}, 2: {} }, new Set([2, 3]))).toEqual({ 1: 2, 3: 1 });
  });
});
