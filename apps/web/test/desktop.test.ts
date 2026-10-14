import { describe, expect, it } from "vitest";
import { bridgeUrl, parseAddress } from "../src/lib/desktop";

describe("parseAddress", () => {
  it("reads host and optional port", () => {
    expect(parseAddress("voice.example.org")).toEqual({ host: "voice.example.org", port: 64738 });
    expect(parseAddress(" voice.example.org:1234 ")).toEqual({ host: "voice.example.org", port: 1234 });
    expect(parseAddress("[2001:db8::1]:999")).toEqual({ host: "2001:db8::1", port: 999 });
    expect(parseAddress("2001:db8::1")).toEqual({ host: "2001:db8::1", port: 64738 });
  });

  it("refuses empty or broken addresses", () => {
    expect(parseAddress("")).toBeNull();
    expect(parseAddress("host:99999")).toBeNull();
    expect(parseAddress("host:abc")).toBeNull();
  });
});

describe("bridgeUrl", () => {
  it("points the client at the local bridge with secret and target", () => {
    const url = new URL(bridgeUrl({ port: 4567, secret: "s3cret" }, { host: "voice.example.org", port: 64738 }));
    expect(url.origin).toBe("ws://127.0.0.1:4567");
    expect(Object.fromEntries(url.searchParams)).toEqual({ secret: "s3cret", host: "voice.example.org", port: "64738" });
  });
});

describe("parseAddress strictness", () => {
  it("accepts only decimal ports", () => {
    expect(parseAddress("host:0x10")).toBeNull();
    expect(parseAddress("host:1e3")).toBeNull();
    expect(parseAddress("host: 80")).toBeNull();
  });
});
