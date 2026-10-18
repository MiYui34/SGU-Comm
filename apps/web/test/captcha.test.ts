import { describe, expect, it } from "vitest";
import { createAccountsApi } from "../src/lib/accounts";
import { solve, zeroBits } from "../src/lib/captcha";
import { initialTextState, manageRequest, parseServerEvent, reduceText } from "../src/lib/textChannels";

const sha256 = async (s: string) => new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(s)));

describe("captcha", () => {
  it("counts leading zero bits", () => {
    expect(zeroBits(new Uint8Array([0x80]))).toBe(0);
    expect(zeroBits(new Uint8Array([0x01]))).toBe(7);
    expect(zeroBits(new Uint8Array([0, 0x40]))).toBe(9);
    expect(zeroBits(new Uint8Array([0, 0]))).toBe(16);
  });

  it("finds a nonce with enough zero bits, as the server checks", async () => {
    const nonce = await solve("salt.mac", 10);
    expect(zeroBits(await sha256(`salt.mac${nonce}`))).toBeGreaterThanOrEqual(10);
    expect(await solve("salt.mac", 0)).toBe("0");
  });
});

describe("the captcha setting", () => {
  it("comes with ready, changes later, and admins set it", () => {
    const s = reduceText(initialTextState, parseServerEvent('{"type":"ready","channels":[],"captcha":"normal"}')!);
    expect(s.captcha).toBe("normal");
    expect(reduceText(s, parseServerEvent('{"type":"captcha","level":"hard"}')!).captcha).toBe("hard");
    expect(reduceText(s, parseServerEvent('{"type":"ready","channels":[]}')!).captcha).toBe("off");
    expect(parseServerEvent('{"type":"captcha","level":"extreme"}')).toBeNull();
    expect(manageRequest({ captcha: "hard" })).toBe('{"type":"set-captcha","level":"hard"}');
  });
});

describe("accounts API with a captcha", () => {
  const fetchWith = (answers: Record<string, [number, unknown]>, bodies: Record<string, unknown>) =>
    (async (url: string, init: RequestInit) => {
      const path = new URL(url).pathname;
      if (init.body) bodies[path] = JSON.parse(String(init.body));
      const [status, body] = answers[path] ?? [404, {}];
      return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
    }) as unknown as typeof fetch;

  it("gets a challenge, or null when off or unknown", async () => {
    const on = { challenge: "c", bits: 18 };
    expect(await createAccountsApi("http://srv", fetchWith({ "/api/challenge": [200, { ...on, expires: 1 }] }, {})).challenge()).toEqual(on);
    expect(await createAccountsApi("http://srv", fetchWith({ "/api/challenge": [200, { bits: 0 }] }, {})).challenge()).toBeNull();
    expect(await createAccountsApi("http://srv", fetchWith({}, {})).challenge()).toBeNull();
  });

  it("sends the proof, and a refused one is an error for guests too", async () => {
    const bodies: Record<string, unknown> = {};
    const api = createAccountsApi("http://srv", fetchWith({ "/api/accounts": [201, { name: "bob" }] }, bodies));
    await api.create("bob", "longpassword", { challenge: "c", nonce: "7" });
    expect(bodies["/api/accounts"]).toEqual({ name: "bob", password: "longpassword", challenge: "c", nonce: "7" });
    const refused = createAccountsApi("http://srv", fetchWith({ "/api/guest": [403, { error: "the browser check did not pass" }] }, bodies));
    await expect(refused.guest("visitor", { challenge: "c", nonce: "8" })).rejects.toThrow("browser check");
    expect(bodies["/api/guest"]).toEqual({ name: "visitor", challenge: "c", nonce: "8" });
  });
});
