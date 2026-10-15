// The Kallisti server's account API. The session is an HttpOnly cookie, so
// every call includes credentials (the API may be on another origin in
// development).
// A logged-in account; temporary: a guest account without a password (the
// server deletes it after a day without a visit).
export type Account = { name: string; temporary: boolean };

// An answer the server refused, with its status.
class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

export const createAccountsApi = (base: string, fetchFn: typeof fetch = fetch) => {
  type Answer = { name?: string; mumble?: boolean; temporary?: boolean };
  const call = async (path: string, body?: unknown): Promise<Answer> => {
    // GET without Content-Type: no CORS preflight needed.
    const res = await fetchFn(`${base}${path}`, {
      credentials: "include",
      ...(body === undefined
        ? { method: "GET" }
        : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }),
    });
    const data = (await res.json().catch(() => ({}))) as Answer & { error?: string };
    if (!res.ok) throw new ApiError(data.error ?? `request failed (${res.status})`, res.status);
    return data;
  };
  return {
    // mumble: a Mumble registration (built-in server), not a web account. It
    // has no session: connect with the name and password.
    login: async (name: string, password: string) => {
      const data = await call("/api/login", { name, password });
      return { name: data.name ?? name, mumble: data.mumble === true };
    },
    create: async (name: string, password: string) =>
      (await call("/api/accounts", { name, password })).name ?? name,
    // A temporary account for a guest; null: join as a classic guest (the
    // server has no guest accounts, too many right now, or no answer).
    // Name problems are errors.
    guest: async (name: string): Promise<Account | null> => {
      try {
        return { name: (await call("/api/guest", { name: name.trim() })).name ?? name.trim(), temporary: true };
      } catch (err) {
        if (err instanceof ApiError && (err.status === 400 || err.status === 409)) throw err;
        return null;
      }
    },
    // false: the server did not confirm (the session cookie may remain).
    logout: async () =>
      call("/api/logout", {}).then(
        () => true,
        () => false,
      ),
    logoutAll: async () => {
      await call("/api/logout-all", {});
    },
    changePassword: async (old: string, next: string) => {
      await call("/api/password", { old, new: next });
    },
    // The logged-in account, or null (also for servers without accounts).
    me: async (): Promise<Account | null> => {
      try {
        const data = await call("/api/me");
        return data.name ? { name: data.name, temporary: data.temporary === true } : null;
      } catch {
        return null;
      }
    },
  };
};

export type AccountsApi = ReturnType<typeof createAccountsApi>;

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
};
