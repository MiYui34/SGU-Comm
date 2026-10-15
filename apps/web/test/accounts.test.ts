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
    expect(await api2.me()).toBe("carol");
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
