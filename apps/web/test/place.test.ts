import { describe, expect, it } from "vitest";
import { loadPlace, placeKey, relinkDirect, savePlace, type Place } from "../src/lib/place";

const memory = () => {
  const m = new Map<string, string>();
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v), m };
};

const place: Place = { session: 7, view: { kind: "text", id: 3 }, voice: 2, thread: 9, selfMute: true, selfDeaf: false };

describe("savePlace and loadPlace", () => {
  it("brings the place back after a refresh, without the old session (it is not ours now)", () => {
    const s = memory();
    savePlace(s, "k", place);
    expect(loadPlace(s, "k")).toEqual({ ...place, session: null });
  });

  it("keeps the session when the page saved it on leaving (a refresh, not a copied tab)", () => {
    const s = memory();
    savePlace(s, "k", place, true);
    expect(loadPlace(s, "k")).toEqual(place);
    savePlace(s, "k", place); // saved again while open: a copy of the tab now is not the same page
    expect(loadPlace(s, "k")?.session).toBeNull();
  });

  it("keeps a direct chat by name and user id", () => {
    const s = memory();
    const dm: Place = { ...place, view: { kind: "dm", session: 4, name: "ann", userId: 12 } };
    savePlace(s, "k", dm);
    expect(loadPlace(s, "k")?.view).toEqual(dm.view);
  });

  it("nothing saved, another key, broken or strange data: no place", () => {
    const s = memory();
    expect(loadPlace(s, "k")).toBeNull();
    savePlace(s, "k", place);
    expect(loadPlace(s, "other")).toBeNull();
    const rest = '"thread":null,"selfMute":false,"selfDeaf":false';
    for (const bad of [
      "",
      "{",
      "null",
      "[]",
      `{"view":{"kind":"text","id":"3"},"voice":null,${rest}}`,
      `{"view":{"kind":"x"},"voice":null,${rest}}`,
      `{"view":{"kind":"dm","session":null},"voice":null,${rest}}`,
      `{"view":{"kind":"voice"},"voice":"2",${rest}}`,
    ]) {
      s.m.set("k", bad);
      expect(loadPlace(s, "k")).toBeNull();
    }
  });

  it("clearing (null) forgets it", () => {
    const s = memory();
    savePlace(s, "k", place);
    savePlace(s, "k", null);
    expect(loadPlace(s, "k")).toBeNull();
  });

  it("no storage, or storage that throws: nothing, without errors", () => {
    const fail = () => {
      throw new Error("x");
    };
    const broken = { getItem: fail, setItem: fail };
    expect(() => savePlace(broken, "k", place)).not.toThrow();
    expect(loadPlace(broken, "k")).toBeNull();
    expect(loadPlace(undefined, "k")).toBeNull();
  });
});

describe("placeKey", () => {
  it("is per site, server and user", () => {
    expect(placeKey("https://a", "", "ann")).not.toBe(placeKey("https://a", "", "bob"));
    expect(placeKey("https://a", "", "ann")).not.toBe(placeKey("https://b", "", "ann"));
    expect(placeKey("https://a", "m:1", "ann")).not.toBe(placeKey("https://a", "m:2", "ann"));
  });
});

describe("relinkDirect", () => {
  const users = [
    { session: 21, name: "ann", userId: 12 },
    { session: 22, name: "guest", userId: null },
  ];
  it("finds the partner's session now: by user id, else by name", () => {
    expect(relinkDirect({ kind: "dm", session: 4, name: "ann", userId: 12 }, users)).toEqual({ kind: "dm", session: 21, name: "ann", userId: 12 });
    expect(relinkDirect({ kind: "dm", session: 4, name: "ann-renamed", userId: 12 }, users)).toMatchObject({ session: 21 });
    expect(relinkDirect({ kind: "dm", session: 5, name: "guest" }, users)).toMatchObject({ session: 22 });
  });

  it("partner offline: no session (the chat stays, by name)", () => {
    expect(relinkDirect({ kind: "dm", session: 4, name: "bob", userId: 99 }, users)).toEqual({ kind: "dm", session: null, name: "bob", userId: 99 });
  });

  it("does not take a registered user's session for a guest of the same name", () => {
    expect(relinkDirect({ kind: "dm", session: 4, name: "ann", userId: null }, users)).toMatchObject({ session: null });
  });

  it("other views stay as they are", () => {
    const v = { kind: "text" as const, id: 3 };
    expect(relinkDirect(v, users)).toBe(v);
  });
});
