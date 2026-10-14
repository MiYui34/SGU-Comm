type Storage = { getItem(key: string): string | null; setItem(key: string, value: string): void };

// localStorage can throw (private mode, full storage) or be missing. Then
// values live only for the session.
export const safeGet = (storage: Storage | undefined, key: string): string | null => {
  try {
    return storage?.getItem(key) ?? null;
  } catch {
    return null;
  }
};

export const safeSet = (storage: Storage | undefined, key: string, value: string): boolean => {
  try {
    if (!storage) return false;
    storage.setItem(key, value);
    return true;
  } catch {
    return false;
  }
};
