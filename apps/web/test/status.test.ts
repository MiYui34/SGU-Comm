import { initialState } from "@mumble-discord/protocol";
import { describe, expect, it } from "vitest";
import { initials } from "../src/lib/avatar";
import { connectionError, proxyUrl } from "../src/lib/status";

describe("connectionError", () => {
  it("is null while connecting or synced and open", () => {
    expect(connectionError(initialState, false)).toBeNull();
    expect(connectionError({ ...initialState, status: "synced" }, false)).toBeNull();
  });

  it("uses the server reason when there is one", () => {
    const s = { ...initialState, status: "rejected" as const, error: "Wrong password" };
    expect(connectionError(s, true)).toBe("Wrong password");
  });

  it("has fallbacks for empty reasons", () => {
    expect(connectionError({ ...initialState, status: "rejected" }, false)).toMatch(/rejected/);
    expect(connectionError({ ...initialState, status: "kicked", error: "" }, false)).toMatch(
      /removed/,
    );
  });

  it("reports a closed socket", () => {
    expect(connectionError({ ...initialState, status: "synced" }, true)).toMatch(/Lost/);
  });
});

describe("proxyUrl", () => {
  it("uses wss on https pages", () => {
    expect(proxyUrl({ protocol: "https:", hostname: "chat.example" })).toBe(
      "wss://chat.example:8080",
    );
    expect(proxyUrl({ protocol: "http:", hostname: "localhost" })).toBe("ws://localhost:8080");
  });
});

describe("initials with emoji", () => {
  it("does not split surrogate pairs", () => {
    expect(initials("🎮gamer")).toBe("🎮");
  });
});
