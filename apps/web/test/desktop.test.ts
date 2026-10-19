import { describe, expect, it } from "vitest";
import { bridgeUrl, certificateProblem, formatFingerprint, isKallisti, kallistiOrigin, kallistiSocket, linuxAgent, parseAddress, registerGlobalPtt } from "../src/lib/desktop";

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

describe("Kallisti servers", () => {
  it("are tried at the address's HTTPS origin, or at the origin typed", () => {
    expect(kallistiOrigin("chat.example")).toBe("https://chat.example");
    expect(kallistiOrigin("chat.example:64738")).toBe("https://chat.example");
    expect(kallistiOrigin(" https://chat.example/ ")).toBe("https://chat.example");
    expect(kallistiOrigin("http://localhost:18083")).toBe("http://localhost:18083");
    expect(kallistiOrigin("")).toBeNull();
  });

  it("answer /api/me with Kallisti's JSON (logged in or not)", async () => {
    const answer = (status: number, body: string, type = "application/json") =>
      (async () => new Response(body, { status, headers: { "Content-Type": type } })) as unknown as typeof fetch;
    expect(await isKallisti("https://a", answer(401, '{"error":"not logged in"}'))).toBe("kallisti");
    expect(await isKallisti("https://a", answer(200, '{"name":"lain","temporary":false}'))).toBe("kallisti");
    expect(await isKallisti("https://a", answer(401, '{"message":"Unauthorized"}'))).toBe("other"); // some other API
    expect(await isKallisti("https://a", answer(404, "nope", "text/html"))).toBe("other");
    const throws = (err: Error) =>
      (async () => {
        throw err;
      }) as unknown as typeof fetch;
    expect(await isKallisti("https://a", throws(new TypeError("refused")))).toBe("other"); // no web server: Mumble
    expect(await isKallisti("https://a", throws(new DOMException("slow", "TimeoutError")))).toBe("unreachable");
  });

  it("are reached on their WebSocket, with the token as a subprotocol", () => {
    expect(kallistiSocket("https://chat.example", "abc")).toEqual({ url: "wss://chat.example/ws", protocols: ["kallisti", "kallisti.session.abc"] });
    expect(kallistiSocket("http://localhost:18083", null)).toEqual({ url: "ws://localhost:18083/ws", protocols: [] });
  });
});

describe("linuxAgent", () => {
  it("is Linux, not Android, macOS or Windows", () => {
    expect(linuxAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15")).toBe(true);
    expect(linuxAgent("Mozilla/5.0 (Linux; Android 14; Pixel 8)")).toBe(false);
    expect(linuxAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")).toBe(false);
    expect(linuxAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64)")).toBe(false);
  });
});
