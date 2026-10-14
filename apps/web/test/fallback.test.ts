import { describe, expect, it } from "vitest";
import { shouldFallBack } from "../src/lib/voice/fallback";

const err = (name: string) => Object.assign(new Error(name), { name });

describe("shouldFallBack", () => {
  it("falls back when a chosen device is gone or busy", () => {
    for (const name of ["OverconstrainedError", "NotFoundError", "NotReadableError"]) {
      expect(shouldFallBack(err(name), "mic-2")).toBe(true);
    }
  });

  it("does not fall back without a chosen device, or when permission is denied", () => {
    expect(shouldFallBack(err("NotFoundError"), null)).toBe(false);
    expect(shouldFallBack(err("NotAllowedError"), "mic-2")).toBe(false);
    expect(shouldFallBack("weird", "mic-2")).toBe(false);
  });
});
