import { describe, expect, it } from "vitest";
import { safeGet, safeSet } from "../src/lib/storage";

const broken = {
  getItem: () => {
    throw new Error("SecurityError");
  },
  setItem: () => {
    throw new Error("QuotaExceededError");
  },
};

describe("safe storage", () => {
  it("reads and writes normally", () => {
    const data = new Map<string, string>();
    const store = { getItem: (k: string) => data.get(k) ?? null, setItem: (k: string, v: string) => void data.set(k, v) };
    expect(safeSet(store, "a", "1")).toBe(true);
    expect(safeGet(store, "a")).toBe("1");
  });

  it("survives a storage that throws", () => {
    expect(safeGet(broken, "a")).toBeNull();
    expect(safeSet(broken, "a", "1")).toBe(false);
  });

  it("survives a missing storage", () => {
    expect(safeGet(undefined, "a")).toBeNull();
    expect(safeSet(undefined, "a", "1")).toBe(false);
  });
});
