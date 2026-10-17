import { describe, expect, it } from "vitest";
import { BIO_MAX, bioToSave } from "../src/lib/bio";

describe("bioToSave", () => {
  it("sends a changed bio, trimmed", () => {
    expect(bioToSave("  hi\nthere \n", "")).toBe("hi\nthere");
    expect(bioToSave("", "old")).toBe(""); // cleared
  });

  it("sends nothing when it did not change", () => {
    expect(bioToSave("same ", "same")).toBeNull();
    const long = "x".repeat(BIO_MAX + 10); // from a Mumble app: not cut
    expect(bioToSave(long, long)).toBeNull();
  });

  it("cuts it to the limit", () => {
    expect(bioToSave("x".repeat(BIO_MAX + 10), "")).toBe("x".repeat(BIO_MAX));
  });
});
