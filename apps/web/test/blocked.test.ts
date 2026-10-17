import { describe, expect, it } from "vitest";
import { isBlocked, parseBlocked, toggleBlocked } from "../src/lib/blocked";

describe("blocked users", () => {
  it("toggles a name, case-insensitive like Mumble names", () => {
    const one = toggleBlocked([], "Bob");
    expect(isBlocked(one, "bob")).toBe(true);
    expect(isBlocked(one, "BOB")).toBe(true);
    expect(isBlocked(one, "alice")).toBe(false);
    expect(toggleBlocked(one, "BOB")).toEqual([]);
  });

  it("never blocks no one (server messages)", () => {
    expect(isBlocked(["bob"], null)).toBe(false);
  });

  it("parses only lists of names", () => {
    expect(parseBlocked('["bob", 3, "Eve"]')).toEqual(["bob", "eve"]);
    expect(parseBlocked("{}")).toEqual([]);
    expect(parseBlocked("not json")).toEqual([]);
    expect(parseBlocked(null)).toEqual([]);
  });
});
