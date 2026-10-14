// A reactive value saved in localStorage. `parse` must accept null and
// broken data (old versions, other tabs, manual edits).
export const persisted = <T>(key: string, parse: (raw: string | null) => T) => {
  let current = $state.raw<T>(parse(localStorage.getItem(key)));
  return {
    get value(): T {
      return current;
    },
    set: (value: T) => {
      current = value;
      localStorage.setItem(key, JSON.stringify(value));
    },
  };
};
