// The Kallisti server's account API. The session is an HttpOnly cookie, so
// every call includes credentials (the API may be on another origin in
// development). The desktop app (session given) cannot use the cookie (it
// is third-party there): it keeps the server's token and sends it.
import type { Challenge, Proof } from "./captcha";
import type { FediLink } from "./fedi";

// A logged-in account; temporary: a guest account without a password (the
// server deletes it after a day without a visit). password false: made
// with a fediverse login, no password yet (undefined: not known). fedi:
// the linked fediverse account.
export type Account = { name: string; temporary: boolean; password?: boolean; fedi?: FediLink | null };

// An answer the server refused, with its status.
class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

export type SessionStore = { get: () => string | null; set: (token: string | null) => void };

export const createAccountsApi = (base: string, fetchFn: typeof fetch = fetch, session?: SessionStore) => {
  type Answer = { name?: string; mumble?: boolean; temporary?: boolean; token?: string } & Partial<Challenge> &
    Partial<StreamKey> & { key?: string; url?: string; handle?: string; password?: boolean; fedi?: FediLink };
  const call = async (path: string, body?: unknown, method = body === undefined ? "GET" : "POST"): Promise<Answer> => {
    const token = session?.get();
    const headers: Record<string, string> = {
      ...(body !== undefined && { "Content-Type": "application/json" }),
      ...(session && { "X-Kallisti-Session": "token" }),
      ...(token && { Authorization: `Bearer ${token}` }),
    };
    // A web GET without Content-Type: no CORS preflight needed.
    const res = await fetchFn(`${base}${path}`, {
      credentials: session ? "omit" : "include",
      method,
      ...(Object.keys(headers).length && { headers }),
      ...(body !== undefined && { body: JSON.stringify(body) }),
    });
    const data = (await res.json().catch(() => ({}))) as Answer & { error?: string };
    // The server no longer knows the token (not: a wrong password).
    if (res.status === 401 && path === "/api/me") session?.set(null);
    if (!res.ok) throw new ApiError(data.error ?? `request failed (${res.status})`, res.status);
    if (data.token) session?.set(data.token);
    return data;
  };
  return {
    // mumble: a Mumble registration (built-in server), not a web account. It
    // has no session: connect with the name and password.
    login: async (name: string, password: string) => {
      const data = await call("/api/login", { name, password });
      return { name: data.name ?? name, mumble: data.mumble === true };
    },
    // A challenge to solve before create and guest (see captcha), or null:
    // none needed (or a server without them).
    challenge: async (): Promise<Challenge | null> => {
      const c = await call("/api/challenge").catch(() => null);
      return c?.challenge && c.bits ? { challenge: c.challenge, bits: c.bits } : null;
    },
    create: async (name: string, password: string, proof?: Proof) =>
      (await call("/api/accounts", { name, password, ...proof })).name ?? name,
    // A temporary account for a guest; null: join as a classic guest (the
    // server has no guest accounts, too many right now, or no answer).
    // Name problems and a failed browser check (403) are errors.
    guest: async (name: string, proof?: Proof): Promise<Account | null> => {
      try {
        return { name: (await call("/api/guest", { name: name.trim(), ...proof })).name ?? name.trim(), temporary: true };
      } catch (err) {
        if (err instanceof ApiError && [400, 403, 409].includes(err.status)) throw err;
        return null;
      }
    },
    // false: the server did not confirm (the session cookie may remain).
    logout: async () =>
      call("/api/logout", {}).then(
        () => (session?.set(null), true),
        () => false,
      ),
    logoutAll: async () => {
      await call("/api/logout-all", {});
      session?.set(null);
    },
    changePassword: async (old: string, next: string) => {
      await call("/api/password", { old, new: next });
    },
    whipUrl: whipUrl(base),
    // Streaming from OBS: off on the server (or for guest accounts), or on
    // with or without a key.
    streamKey: async (): Promise<StreamKey> => {
      const data = await call("/api/stream-key").catch(() => ({}) as Answer);
      return { enabled: data.enabled === true, exists: data.exists === true };
    },
    // A new key (the old one stops working): shown this once.
    newStreamKey: async () => (await call("/api/stream-key", {})).key ?? "",
    removeStreamKey: async () => {
      await call("/api/stream-key", undefined, "DELETE");
    },
    // Fediverse logins (see lib/fedi): on for this server?
    fediEnabled: async () => (await call("/api/fedi").catch(() => ({}) as Answer & { enabled?: boolean })).enabled === true,
    // The instance's page to go to, for a login (or a link: with the
    // account's password, "" for one without).
    fediStart: async (instance: string, linkPassword?: string) => {
      const { url } = await call("/api/fedi/start", { instance, ...(linkPassword !== undefined && { link: true, password: linkPassword }) });
      if (!url) throw new Error("No answer from the server. Please try again.");
      return url;
    },
    // After a login with an unlinked fediverse account: the suggested name
    // and the handle; null: none (expired).
    fediSignup: async (): Promise<{ name: string; handle: string } | null> => {
      const data = await call("/api/fedi/signup").catch(() => null);
      return data ? { name: data.name ?? "", handle: data.handle ?? "" } : null;
    },
    fediCreate: async (name: string, proof?: Proof) => (await call("/api/fedi/signup", { name, ...proof })).name ?? name,
    fediUnlink: async () => {
      await call("/api/fedi/unlink", {});
    },
    fediPublic: async (on: boolean) => {
      await call("/api/fedi/public", { on });
    },
    // The logged-in account, or null (also for servers without accounts).
    me: async (): Promise<Account | null> => {
      try {
        const data = await call("/api/me");
        return data.name ? { name: data.name, temporary: data.temporary === true, password: data.password, fedi: data.fedi } : null;
      } catch {
        return null;
      }
    },
  };
};

export type AccountsApi = ReturnType<typeof createAccountsApi>;

export type StreamKey = { enabled: boolean; exists: boolean };

// Where OBS sends a stream (WHIP), on the Kallisti server.
export const whipUrl = (base: string) => `${base}/api/whip`;

// The login screen opens on "Log in" when a guest tried an account's name.
export const modeForError = (error: string | null): "guest" | "login" =>
  error?.includes("belongs to an account") ? "login" : "guest";

// What a logged-in user can do from the settings (temporary: a guest
// account, which gets its first password without an old one).
export type AccountActions = {
  temporary: boolean;
  logout: () => Promise<void>;
  logoutAll: () => Promise<void>;
  changePassword: (old: string, next: string) => Promise<void>;
  // Streaming from OBS (see the server's whip package).
  whipUrl: string;
  streamKey: () => Promise<StreamKey>;
  newStreamKey: () => Promise<string>;
  removeStreamKey: () => Promise<void>;
  // false: an account made with a fediverse login, without a password yet
  // (it sets its first one without an old one).
  hasPassword: boolean;
  // The linked fediverse account (see lib/fedi). fediLink goes to the
  // instance (with the account's password; "" without one).
  fedi: FediLink | null;
  fediEnabled: () => Promise<boolean>;
  fediLink: (instance: string, password: string) => Promise<void>;
  fediUnlink: () => Promise<void>;
  fediPublic: (on: boolean) => Promise<void>;
};
