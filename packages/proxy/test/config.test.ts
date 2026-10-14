import { describe, expect, it } from "vitest";
import { isOriginAllowed, parseConfig } from "../src/config.ts";

describe("parseConfig", () => {
  it("uses defaults for the optional settings", () => {
    expect(parseConfig({ MUMBLE_HOST: "voice.example.org", ALLOWED_ORIGINS: "*" })).toEqual({
      ok: true,
      config: {
        mumbleHost: "voice.example.org",
        mumblePort: 64738,
        listenPort: 8080,
        allowedOrigins: ["*"],
        rejectUnauthorized: true,
        maxConnections: 100,
      },
    });
  });

  it("requires ALLOWED_ORIGINS", () => {
    expect(parseConfig({ MUMBLE_HOST: "x" })).toEqual({
      ok: false,
      error: "ALLOWED_ORIGINS is required (use * to allow every origin)",
    });
  });

  it("reads all options", () => {
    const result = parseConfig({
      MUMBLE_HOST: "localhost",
      MUMBLE_PORT: "1234",
      LISTEN_PORT: "9000",
      ALLOWED_ORIGINS: "https://a.example, http://localhost:5173",
      MUMBLE_INSECURE_TLS: "1",
      MAX_CONNECTIONS: "5",
    });
    expect(result).toEqual({
      ok: true,
      config: {
        mumbleHost: "localhost",
        mumblePort: 1234,
        listenPort: 9000,
        allowedOrigins: ["https://a.example", "http://localhost:5173"],
        rejectUnauthorized: false,
        maxConnections: 5,
      },
    });
  });

  it("requires MUMBLE_HOST", () => {
    expect(parseConfig({})).toEqual({ ok: false, error: "MUMBLE_HOST is required" });
  });

  it("rejects bad port numbers", () => {
    expect(parseConfig({ MUMBLE_HOST: "x", ALLOWED_ORIGINS: "*", MUMBLE_PORT: "abc" })).toEqual({
      ok: false,
      error: "MUMBLE_PORT must be a port number",
    });
    expect(parseConfig({ MUMBLE_HOST: "x", ALLOWED_ORIGINS: "*", LISTEN_PORT: "70000" })).toEqual({
      ok: false,
      error: "LISTEN_PORT must be a port number",
    });
  });

  it("rejects a bad connection limit", () => {
    expect(parseConfig({ MUMBLE_HOST: "x", ALLOWED_ORIGINS: "*", MAX_CONNECTIONS: "0" })).toEqual({
      ok: false,
      error: "MAX_CONNECTIONS must be a positive integer",
    });
  });
});

describe("isOriginAllowed", () => {
  it("allows every origin with *", () => {
    expect(isOriginAllowed(["*"], "https://any.example")).toBe(true);
    expect(isOriginAllowed(["*"], undefined)).toBe(true);
  });

  it("allows nothing when the list is empty", () => {
    expect(isOriginAllowed([], "https://a.example")).toBe(false);
  });

  it("allows only listed origins otherwise", () => {
    const list = ["https://a.example"];
    expect(isOriginAllowed(list, "https://a.example")).toBe(true);
    expect(isOriginAllowed(list, "https://b.example")).toBe(false);
    expect(isOriginAllowed(list, undefined)).toBe(false);
  });
});
