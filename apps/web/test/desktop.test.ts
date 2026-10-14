import { describe, expect, it } from "vitest";
import { bridgeUrl, certificateProblem, formatFingerprint, parseAddress, registerGlobalPtt } from "../src/lib/desktop";

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

describe("registerGlobalPtt", () => {
  it("registers the key and reports press and release", async () => {
    const calls: string[] = [];
    let handler: ((e: { state: "Pressed" | "Released" }) => void) | undefined;
    const api = {
      register: async (key: string, h: typeof handler) => {
        calls.push(`register ${key}`);
        handler = h;
      },
      unregister: async (key: string) => {
        calls.push(`unregister ${key}`);
      },
    };
    const states: boolean[] = [];
    const stop = await registerGlobalPtt("Backquote", (down) => states.push(down), api);
    handler?.({ state: "Pressed" });
    handler?.({ state: "Released" });
    stop();
    // stop() also releases: a key held while unregistering must not stay on.
    expect(states).toEqual([true, false, false]);
    expect(calls).toEqual(["register Backquote", "unregister Backquote"]);
  });
});

describe("certificateProblem", () => {
  it("reads the bridge's close reasons", () => {
    expect(certificateProblem("certificate-unknown sha256:ab12")).toEqual({ kind: "unknown", fingerprint: "sha256:ab12" });
    expect(certificateProblem("certificate-changed sha256:cd34")).toEqual({ kind: "changed", fingerprint: "sha256:cd34" });
    expect(certificateProblem("cannot reach x:1: refused")).toBeNull();
    expect(certificateProblem("")).toBeNull();
  });
});

describe("formatFingerprint", () => {
  it("groups the hex digits for reading aloud", () => {
    expect(formatFingerprint("sha256:ab12cd34ef56")).toBe("AB:12:CD:34:EF:56");
  });
});
