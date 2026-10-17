import { describe, expect, it } from "vitest";
import { tabTitle } from "../src/lib/title";

describe("tabTitle", () => {
  it("shows mentions and direct messages not read, as Discord", () => {
    expect(tabTitle("Kallisti", 0)).toBe("Kallisti");
    expect(tabTitle("Kallisti", 3)).toBe("(3) Kallisti");
    expect(tabTitle("My Server", 120)).toBe("(99+) My Server");
  });
});
