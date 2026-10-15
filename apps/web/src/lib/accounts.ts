// The Kallisti server's account API. The session is an HttpOnly cookie, so
// every call includes credentials (the API may be on another origin in
// development).
export const createAccountsApi = (base: string, fetchFn: typeof fetch = fetch) => {
  const call = async (path: string, body?: unknown): Promise<{ name?: string; mumble?: boolean }> => {
    // GET without Content-Type: no CORS preflight needed.
    const res = await fetchFn(`${base}${path}`, {
      credentials: "include",
      ...(body === undefined
        ? { method: "GET" }
        : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }),
    });
    const data = (await res.json().catch(() => ({}))) as { name?: string; mumble?: boolean; error?: string };
    if (!res.ok) throw new Error(data.error ?? `request failed (${res.status})`);
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
    me: async () => {
      try {
        return (await call("/api/me")).name ?? null;
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

// What a logged-in user can do from the settings.
export type AccountActions = {
  logout: () => Promise<void>;
  logoutAll: () => Promise<void>;
  changePassword: (old: string, next: string) => Promise<void>;
};
