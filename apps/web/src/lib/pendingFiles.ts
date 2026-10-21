// A file picked for a chat but not sent yet (see ChatPane), kept per chat
// over reconnects (in memory) and reloads (in IndexedDB): on Android the
// file picker can send the app away long enough to lose the page.

export type Pending = { file: File; spoiler: boolean };
export type StoredFile = Pending & { key: string; at: number };

// The storage, injectable for tests. Any call may fail (private mode, full
// storage): then the files live in memory only.
export type FileStore = {
  get(key: string): Promise<StoredFile | undefined>;
  put(row: StoredFile): Promise<void>;
  delete(key: string): Promise<void>;
  all(): Promise<StoredFile[]>;
};

export const PENDING_TTL_MS = 24 * 60 * 60 * 1000;
export const PENDING_CAP = 5;
// With no known upload limit, larger files are not stored.
const FALLBACK_MAX = 50 * 1024 * 1024;

// Rows to delete: the expired ones and all but the newest few.
export const staleKeys = (rows: readonly { key: string; at: number }[], now: number): string[] => {
  const fresh = rows.filter((r) => now - r.at <= PENDING_TTL_MS).sort((a, b) => b.at - a.at);
  const keep = new Set(fresh.slice(0, PENDING_CAP).map((r) => r.key));
  return rows.filter((r) => !keep.has(r.key)).map((r) => r.key);
};

export type PendingFiles = ReturnType<typeof createPendingFiles>;

// origin: the server and user (chat keys repeat between servers).
export const createPendingFiles = (store: FileStore | null, origin: string, now = Date.now) => {
  const memory = new Map<string, Pending>();
  const key = (chat: string) => `${origin} ${chat}`;
  // Store calls one after another (a delete never overtakes the put before it).
  let queue: Promise<unknown> = Promise.resolve();
  const later = <T>(f: (s: FileStore) => Promise<T>): Promise<T | undefined> => {
    const run = queue.then(() => (store ? f(store) : undefined)).catch(() => undefined);
    queue = run;
    return run;
  };
  const prune = (s: FileStore) =>
    s.all().then((rows) => Promise.all(staleKeys(rows, now()).map((k) => s.delete(k))));
  void later(prune);

  return {
    // Picked or changed (spoiler). max: the server's upload limit.
    set(chat: string, pending: Pending, max: number | undefined) {
      memory.set(chat, pending);
      const row = { key: key(chat), at: now(), ...pending };
      void later(async (s) => {
        if (pending.file.size > (max ?? FALLBACK_MAX)) return s.delete(row.key);
        await s.put(row);
        await prune(s);
      });
    },
    // Sent or discarded; with `file`, only while it is still that file (an
    // upload that ends after another file was picked).
    clear(chat: string, file?: File) {
      if (file && memory.get(chat)?.file !== file) return;
      memory.delete(chat);
      void later((s) => s.delete(key(chat)));
    },
    async get(chat: string): Promise<Pending | undefined> {
      const known = memory.get(chat);
      if (known) return known;
      const row = await later(async (s) => {
        const found = await s.get(key(chat));
        if (!found || now() - found.at <= PENDING_TTL_MS) return found;
        await s.delete(found.key);
        return undefined;
      });
      if (memory.has(chat) || !row) return memory.get(chat);
      const pending = { file: row.file, spoiler: row.spoiler };
      memory.set(chat, pending);
      return pending;
    },
    // All store calls so far done (tests).
    settled: () => queue.then(() => undefined),
  };
};

// The browser's IndexedDB, or null without it.
export const indexedDbStore = (name = "kallisti-pending-files"): FileStore | null => {
  if (typeof indexedDB === "undefined") return null;
  const STORE = "files";
  let db: Promise<IDBDatabase> | null = null;
  const open = () =>
    (db ??= new Promise<IDBDatabase>((resolve, reject) => {
      const req = indexedDB.open(name, 1);
      req.onupgradeneeded = () => req.result.createObjectStore(STORE, { keyPath: "key" });
      req.onsuccess = () => {
        // Closed by the browser (or a newer version elsewhere): open again.
        req.result.onclose = req.result.onversionchange = () => {
          req.result.close();
          db = null;
        };
        resolve(req.result);
      };
      req.onerror = () => reject(req.error);
    }).catch((err) => {
      db = null; // try again next time
      throw err;
    }));
  const run = async <T>(mode: IDBTransactionMode, f: (s: IDBObjectStore) => IDBRequest): Promise<T> => {
    const tx = (await open()).transaction(STORE, mode);
    const req = f(tx.objectStore(STORE));
    return new Promise<T>((resolve, reject) => {
      tx.oncomplete = () => resolve(req.result as T);
      tx.onerror = tx.onabort = () => reject(tx.error ?? req.error);
    });
  };
  return {
    get: (key) => run("readonly", (s) => s.get(key)),
    put: (row) => run<unknown>("readwrite", (s) => s.put(row)).then(() => undefined),
    delete: (key) => run<unknown>("readwrite", (s) => s.delete(key)).then(() => undefined),
    all: () => run("readonly", (s) => s.getAll()),
  };
};
