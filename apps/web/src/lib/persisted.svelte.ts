import { safeGet, safeSet } from "./storage";

const storage = () => (typeof localStorage === "undefined" ? undefined : localStorage);

// A reactive value saved in localStorage, synced between tabs. `parse` must
// accept null and broken data (old versions, other tabs, manual edits).
// When storage fails, the value still works for this session.
export const persisted = <T>(key: string, parse: (raw: string | null) => T) => {
  let current = $state.raw<T>(parse(safeGet(storage(), key)));
  if (typeof window !== "undefined") {
    window.addEventListener("storage", (event) => {
      if (event.key === key) current = parse(event.newValue);
    });
  }
  return {
    get value(): T {
      return current;
    },
    set: (value: T) => {
      current = value;
      safeSet(storage(), key, JSON.stringify(value));
    },
  };
};
