// A file picked for a chat but not sent yet (see ChatPane), kept per chat
// over reconnects (in memory) and reloads (in IndexedDB): on Android the
// file picker can send the app away long enough to lose the page.

export type Pending = { file: File; spoiler: boolean };
// A send of a pending file: in flight, or failed (the file stays).
export type SendStatus = { sending: true } | { sending: false; error: string };
// nonce: the id each send of this file goes with (see sendRequest), so a
// retry cannot post it twice; missing in rows from before.
type Kept = Pending & { nonce: string };
export type StoredFile = Pending & { key: string; at: number; nonce?: string };

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
export const createPendingFiles = (store: FileStore | null, origin: string, now = Date.now, newNonce = (): string => crypto.randomUUID()) => {
  const memory = new Map<string, Kept>();
  // Sends by chat, in memory only: a pane after a reconnect sees a send
  // the old one started, and cannot start it again.
  const sends = new Map<string, SendStatus & { file: File }>();
  const watchers = new Set<() => void>();
  const changed = () => watchers.forEach((fn) => fn());
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
  // A failed send's error goes with its file; a send in flight stays.
  const drop = (chat: string, file?: File) => {
    if (file && memory.get(chat)?.file !== file) return;
    memory.delete(chat);
    if (sends.get(chat)?.sending === false) sends.delete(chat);
    void later((s) => s.delete(key(chat)));
  };
  const status = (chat: string, file: File): SendStatus | undefined => {
    const s = sends.get(chat);
    return s?.file !== file ? undefined : s.sending ? { sending: true } : { sending: false, error: s.error };
  };

  return {
    // Picked or changed (spoiler). max: the server's upload limit.
    set(chat: string, { file, spoiler }: Pending, max: number | undefined) {
      const before = memory.get(chat);
      const kept = { file, spoiler, nonce: before?.file === file ? before.nonce : newNonce() };
      memory.set(chat, kept);
      const row = { key: key(chat), at: now(), ...kept };
      void later(async (s) => {
        if (file.size > (max ?? FALLBACK_MAX)) return s.delete(row.key);
        await s.put(row);
        await prune(s);
      });
    },
    // Sent or discarded; with `file`, only while it is still that file (an
    // upload that ends after another file was picked).
    clear(chat: string, file?: File) {
      drop(chat, file);
      changed();
    },
    // Still waiting in this chat (in memory).
    holds: (chat: string, file: File) => memory.get(chat)?.file === file,
    status,
    // Sends the file with run, which settles once the server confirmed the
    // message (or failed): only then the file goes. False: not sent (failed,
    // or this file is sending already).
    async send(chat: string, file: File, run: (nonce: string) => Promise<void>): Promise<boolean> {
      if (status(chat, file)?.sending) return false;
      sends.set(chat, { file, sending: true });
      changed();
      const mine = () => sends.get(chat)?.file === file;
      const kept = memory.get(chat);
      try {
        await run(kept?.file === file ? kept.nonce : newNonce());
        if (mine()) sends.delete(chat);
        drop(chat, file);
        return true;
      } catch (err) {
        if (mine()) sends.set(chat, { file, sending: false, error: err instanceof Error ? err.message : String(err) });
        return false;
      } finally {
        changed();
      }
    },
    // Calls fn on each change of the sends (and on clear); returns the way out.
    watch(fn: () => void) {
      watchers.add(fn);
      return () => void watchers.delete(fn);
    },
    async get(chat: string): Promise<Pending | undefined> {
      const shown = (k: Kept | undefined) => k && { file: k.file, spoiler: k.spoiler };
      if (memory.has(chat)) return shown(memory.get(chat));
      const row = await later(async (s) => {
        const found = await s.get(key(chat));
        if (!found || now() - found.at <= PENDING_TTL_MS) return found;
        await s.delete(found.key);
        return undefined;
      });
      if (memory.has(chat) || !row) return shown(memory.get(chat));
      const kept = { file: row.file, spoiler: row.spoiler, nonce: row.nonce ?? newNonce() };
      memory.set(chat, kept);
      return shown(kept);
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
