// The chosen microphone cannot be used (unplugged, busy): use the default
// one. A denied permission is not a device problem, so no fallback then.
const DEVICE_ERRORS = ["OverconstrainedError", "NotFoundError", "NotReadableError"];

export const shouldFallBack = (err: unknown, deviceId: string | null): boolean =>
  deviceId !== null && err instanceof Error && DEVICE_ERRORS.includes(err.name);
