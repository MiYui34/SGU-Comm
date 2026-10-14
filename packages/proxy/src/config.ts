export type Config = {
  mumbleHost: string;
  mumblePort: number;
  listenPort: number;
  // "*" allows every origin (development only).
  allowedOrigins: string[];
  // Most Mumble servers use self-signed certificates.
  rejectUnauthorized: boolean;
  maxConnections: number;
};

type Env = Record<string, string | undefined>;
type Result = { ok: true; config: Config } | { ok: false; error: string };

const parsePort = (value: string | undefined, fallback: number): number | null => {
  if (value === undefined) return fallback;
  const n = Number(value);
  return Number.isInteger(n) && n > 0 && n < 65536 ? n : null;
};

export const parseConfig = (env: Env): Result => {
  if (!env.MUMBLE_HOST) return { ok: false, error: "MUMBLE_HOST is required" };
  const mumblePort = parsePort(env.MUMBLE_PORT, 64738);
  if (mumblePort === null) return { ok: false, error: "MUMBLE_PORT must be a port number" };
  const listenPort = parsePort(env.LISTEN_PORT, 8080);
  if (listenPort === null) return { ok: false, error: "LISTEN_PORT must be a port number" };
  const allowedOrigins = (env.ALLOWED_ORIGINS ?? "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
  if (allowedOrigins.length === 0)
    return { ok: false, error: "ALLOWED_ORIGINS is required (use * to allow every origin)" };
  const maxConnections = Number(env.MAX_CONNECTIONS ?? 100);
  if (!Number.isInteger(maxConnections) || maxConnections < 1)
    return { ok: false, error: "MAX_CONNECTIONS must be a positive integer" };
  return {
    ok: true,
    config: {
      mumbleHost: env.MUMBLE_HOST,
      mumblePort,
      listenPort,
      allowedOrigins,
      rejectUnauthorized: env.MUMBLE_INSECURE_TLS !== "1",
      maxConnections,
    },
  };
};

export const isOriginAllowed = (allowed: readonly string[], origin: string | undefined) =>
  allowed.includes("*") || (origin !== undefined && allowed.includes(origin));
