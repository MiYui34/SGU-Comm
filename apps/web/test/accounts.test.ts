import { describe, expect, it } from "vitest";
import { createAccountsApi, modeForError } from "../src/lib/accounts";

type Call = { url: string; init: RequestInit };
const fakeFetch = (status: number, body: unknown, calls: Call[]) =>
  (async (url: string, init: RequestInit) => {
    calls.push({ url, init });
    // 204 has no body, like the real server's answer.
    return new Response(status === 204 ? null : JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  }) as unknown as typeof fetch;

describe("accounts API", () => {
  it("logs in with the cookie included", async () => {
    const calls: Call[] = [];
    const api = createAccountsApi("http://srv:8080", fakeFetch(200, { name: "Alice" }, calls));
    expect(await api.login("alice", "secret pw")).toEqual({ name: "Alice", mumble: false });
    expect(calls[0]?.url).toBe("http://srv:8080/api/login");
    expect(calls[0]?.init).toMatchObject({ method: "POST", credentials: "include" });
    expect(JSON.parse(String(calls[0]?.init.body))).toEqual({ name: "alice", password: "secret pw" });
  });

  it("tells a Mumble registration's login apart", async () => {
    const api = createAccountsApi("http://srv", fakeFetch(200, { name: "SuperUser", mumble: true }, []));
    expect(await api.login("superuser", "pw")).toEqual({ name: "SuperUser", mumble: true });
  });

  it("creates an account", async () => {
    const calls: Call[] = [];
    const api = createAccountsApi("http://srv", fakeFetch(201, { name: "bob" }, calls));
    expect(await api.create("bob", "longpassword")).toBe("bob");
    expect(calls[0]?.url).toBe("http://srv/api/accounts");
  });

  it("throws the server's error message", async () => {
    const api = createAccountsApi("http://srv", fakeFetch(401, { error: "wrong name or password" }, []));
    await expect(api.login("x", "y")).rejects.toThrow("wrong name or password");
  });

  it("returns null from me when not logged in", async () => {
    const api = createAccountsApi("http://srv", fakeFetch(401, { error: "not logged in" }, []));
    expect(await api.me()).toBeNull();
    const api2 = createAccountsApi("http://srv", fakeFetch(200, { name: "carol" }, []));
    expect(await api2.me()).toEqual({ name: "carol", temporary: false });
    const api3 = createAccountsApi("http://srv", fakeFetch(200, { name: "visitor", temporary: true }, []));
    expect(await api3.me()).toEqual({ name: "visitor", temporary: true });
  });

  it("joins as a guest account where the server has them", async () => {
    const calls: Call[] = [];
    const api = createAccountsApi("http://srv", fakeFetch(201, { name: "visitor", temporary: true }, calls));
    expect(await api.guest(" visitor")).toEqual({ name: "visitor", temporary: true });
    expect(calls[0]?.url).toBe("http://srv/api/guest");
    // No guest accounts (an external Murmur): a classic guest.
    const none = createAccountsApi("http://srv", fakeFetch(404, {}, []));
    expect(await none.guest("visitor")).toBeNull();
    const taken = createAccountsApi("http://srv", fakeFetch(409, { error: "this name is already taken" }, []));
    await expect(taken.guest("visitor")).rejects.toThrow("this name is already taken");
    // Too many guests, or no answer: a classic guest still works.
    for (const status of [429, 503]) {
      const busy = createAccountsApi("http://srv", fakeFetch(status, { error: "busy" }, []));
      expect(await busy.guest("visitor")).toBeNull();
    }
    const offline = createAccountsApi("http://srv", (async () => {
      throw new TypeError("Failed to fetch");
    }) as unknown as typeof fetch);
    expect(await offline.guest("visitor")).toBeNull();
  });

  it("treats a server without accounts as not logged in", async () => {
    const api = createAccountsApi("http://srv", (async () => {
      throw new TypeError("Failed to fetch");
    }) as unknown as typeof fetch);
    expect(await api.me()).toBeNull();
  });
});

describe("modeForError", () => {
  it("opens the login tab when a name belongs to an account", () => {
    expect(modeForError("This name belongs to an account. Log in to use it.")).toBe("login");
    expect(modeForError("This name is already in use.")).toBe("guest");
    expect(modeForError(null)).toBe("guest");
  });
});

describe("password change and logout everywhere", () => {
  it("calls the API", async () => {
    const calls: Call[] = [];
    const api = createAccountsApi("http://srv", fakeFetch(204, {}, calls));
    await api.changePassword("old pw", "new password");
    await api.logoutAll();
    expect(calls.map((c) => c.url)).toEqual(["http://srv/api/password", "http://srv/api/logout-all"]);
    expect(JSON.parse(String(calls[0]?.init.body))).toEqual({ old: "old pw", new: "new password" });
  });

  it("reports a failed logout", async () => {
    const api = createAccountsApi("http://srv", fakeFetch(500, { error: "boom" }, []));
    expect(await api.logout()).toBe(false);
  });
});

describe("the desktop app's session token", () => {
  const store = () => {
    let token: string | null = null;
    return { get: () => token, set: (t: string | null) => (token = t) };
  };

  it("asks for a token, keeps it and sends it, without cookies", async () => {
    const calls: Call[] = [];
    const session = store();
    const api = createAccountsApi("https://chat.example", fakeFetch(200, { name: "lain", token: "abc" }, calls), session);
    await api.login("lain", "secret pw");
    expect(session.get()).toBe("abc");
    expect(calls[0]?.init.credentials).toBe("omit");
    expect((calls[0]?.init.headers as Record<string, string>)["X-Kallisti-Session"]).toBe("token");
    await api.me();
    expect((calls[1]?.init.headers as Record<string, string>).Authorization).toBe("Bearer abc");
  });

  it("forgets it on logout and when the server no longer knows it", async () => {
    const session = store();
    session.set("abc");
    await createAccountsApi("https://chat.example", fakeFetch(204, null, []), session).logout();
    expect(session.get()).toBeNull();
    session.set("abc");
    await createAccountsApi("https://chat.example", fakeFetch(401, { error: "not logged in" }, []), session).me();
    expect(session.get()).toBeNull();
  });
});
