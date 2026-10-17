import { describe, expect, it } from "vitest";
import { jumpStep } from "../src/lib/jump";

describe("jumpStep", () => {
  const base = { loaded: true, more: true, loading: false, pages: 0 };

  it("jumps to a loaded message", () => {
    expect(jumpStep(base)).toBe("jump");
  });

  it("loads older pages until the message is there", () => {
    expect(jumpStep({ ...base, loaded: false })).toBe("load");
    expect(jumpStep({ ...base, loaded: false, loading: true })).toBe("wait");
  });

  it("gives up without older pages or after the page limit", () => {
    expect(jumpStep({ ...base, loaded: false, more: false })).toBe("give-up");
    expect(jumpStep({ ...base, loaded: false, pages: 10 })).toBe("give-up");
  });
});
