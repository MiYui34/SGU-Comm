// The Kallisti server's account API. The session is an HttpOnly cookie, so
// every call includes credentials (the API may be on another origin in
// development).
export const createAccountsApi = (base: string, fetchFn: typeof fetch = fetch) => {
  const call = async (path: string, body?: unknown): Promise<{ name?: string }> => {
    // GET without Content-Type: no CORS preflight needed.
    const res = await fetchFn(`${base}${path}`, {
      credentials: "include",
      ...(body === undefined
        ? { method: "GET" }
        : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }),
    });
    const data = (await res.json().catch(() => ({}))) as { name?: string; error?: string };
    if (!res.ok) throw new Error(data.error ?? `request failed (${res.status})`);
    return data;
  };
  return {
    login: async (name: string, password: string) => (await call("/api/login", { name, password })).name ?? name,
    create: async (name: string, password: string) =>
      (await call("/api/accounts", { name, password })).name ?? name,
    logout: async () => {
      await call("/api/logout", {}).catch(() => {});
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
