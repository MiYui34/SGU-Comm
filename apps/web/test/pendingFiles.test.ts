import { describe, expect, it } from "vitest";
import { createPendingFiles, staleKeys, PENDING_CAP, PENDING_TTL_MS, type FileStore, type StoredFile } from "../src/lib/pendingFiles";

const memoryStore = () => {
  const rows = new Map<string, StoredFile>();
  const store: FileStore = {
    get: async (key) => rows.get(key),
    put: async (row) => void rows.set(row.key, row),
    delete: async (key) => void rows.delete(key),
    all: async () => [...rows.values()],
  };
  return { rows, store };
};
const broken: FileStore = {
  get: () => Promise.reject(new Error("no")),
  put: () => Promise.reject(new Error("quota")),
  delete: () => Promise.reject(new Error("no")),
  all: () => Promise.reject(new Error("no")),
};
const file = (name = "a.png", size = 3) => new File(["x".repeat(size)], name, { type: "image/png" });
const H = 60 * 60 * 1000;

describe("staleKeys", () => {
  it("drops expired rows and keeps only the newest few", () => {
    const now = 100 * H;
    const rows = [
      { key: "old", at: now - PENDING_TTL_MS - 1 },
      ...Array.from({ length: PENDING_CAP + 1 }, (_, i) => ({ key: `k${i}`, at: now - i })),
    ];
    expect(staleKeys(rows, now).sort()).toEqual(["k" + PENDING_CAP, "old"].sort());
  });
});

describe("createPendingFiles", () => {
  it("keeps a file per chat in memory and in the store, keyed by origin", async () => {
    const { rows, store } = memoryStore();
    const files = createPendingFiles(store, "https://a.example", () => 5);
    const f = file();
    files.set("text 1", { file: f, spoiler: true }, 100);
    expect(await files.get("text 1")).toEqual({ file: f, spoiler: true });
    await files.settled();
    expect(rows.get("https://a.example text 1")).toEqual({ key: "https://a.example text 1", at: 5, file: f, spoiler: true });
    expect(await files.get("text 2")).toBeUndefined();
  });

  it("restores from the store after a reload (a new instance), not from another origin", async () => {
    const { store } = memoryStore();
    const f = file();
    const before = createPendingFiles(store, "https://a.example", () => 5);
    before.set("dm bob", { file: f, spoiler: false }, undefined);
    await before.settled();
    expect(await createPendingFiles(store, "https://a.example", () => 6).get("dm bob")).toEqual({ file: f, spoiler: false });
    expect(await createPendingFiles(store, "https://b.example", () => 6).get("dm bob")).toBeUndefined();
  });

  it("does not restore an expired file and removes it", async () => {
    const { rows, store } = memoryStore();
    const before = createPendingFiles(store, "o", () => 0);
    before.set("c", { file: file(), spoiler: false }, 100);
    await before.settled();
    const after = createPendingFiles(store, "o", () => PENDING_TTL_MS + 1);
    expect(await after.get("c")).toBeUndefined();
    await after.settled();
    expect(rows.size).toBe(0);
  });

  it("stores only files within the upload limit (memory keeps them)", async () => {
    const { rows, store } = memoryStore();
    const files = createPendingFiles(store, "o", () => 0);
    files.set("c", { file: file("small.png", 3), spoiler: false }, 100);
    await files.settled();
    const big = file("big.png", 10);
    files.set("c", { file: big, spoiler: false }, 5);
    await files.settled();
    expect(rows.size).toBe(0); // the smaller one stored before is gone too
    expect((await files.get("c"))?.file).toBe(big);
  });

  it("caps the stored files", async () => {
    const { rows, store } = memoryStore();
    let t = 0;
    const files = createPendingFiles(store, "o", () => t++);
    for (let i = 0; i <= PENDING_CAP; i++) files.set(`c${i}`, { file: file(), spoiler: false }, 100);
    await files.settled();
    expect(rows.size).toBe(PENDING_CAP);
    expect(rows.has("o c0")).toBe(false);
  });

  it("clear takes it out of memory and the store; with a file only if it is still that file", async () => {
    const { rows, store } = memoryStore();
    const files = createPendingFiles(store, "o", () => 0);
    const a = file("a.png");
    const b = file("b.png");
    files.set("c", { file: b, spoiler: false }, 100);
    files.clear("c", a);
    await files.settled();
    expect((await files.get("c"))?.file).toBe(b);
    expect(rows.size).toBe(1);
    files.clear("c", b);
    await files.settled();
    expect(await files.get("c")).toBeUndefined();
    expect(rows.size).toBe(0);
  });

  it("a cleared chat is not brought back from the store", async () => {
    const { store } = memoryStore();
    const before = createPendingFiles(store, "o", () => 0);
    before.set("c", { file: file(), spoiler: false }, 100);
    await before.settled();
    const after = createPendingFiles(store, "o", () => 1);
    after.clear("c");
    expect(await after.get("c")).toBeUndefined();
  });

  it("works from memory when the store fails or is missing", async () => {
    for (const store of [broken, null]) {
      const files = createPendingFiles(store, "o", () => 0);
      const f = file();
      files.set("c", { file: f, spoiler: false }, 100);
      await files.settled();
      expect((await files.get("c"))?.file).toBe(f);
      expect(await files.get("other")).toBeUndefined();
      files.clear("c");
      expect(await files.get("c")).toBeUndefined();
    }
  });
});
