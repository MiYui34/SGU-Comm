// Users whose messages are hidden for me (Discord's "block"): local, by
// lower-case name like the per-user voice settings (names are unique
// without case on Mumble).
export type Blocked = readonly string[];

export const isBlocked = (list: Blocked, name: string | null) => name !== null && list.includes(name.toLowerCase());

export const toggleBlocked = (list: Blocked, name: string): Blocked => {
  const key = name.toLowerCase();
  return list.includes(key) ? list.filter((n) => n !== key) : [...list, key];
};

export const parseBlocked = (raw: string | null): Blocked => {
  try {
    const data: unknown = JSON.parse(raw ?? "null");
    return Array.isArray(data) ? data.filter((n): n is string => typeof n === "string").map((n) => n.toLowerCase()) : [];
  } catch {
    return [];
  }
};
