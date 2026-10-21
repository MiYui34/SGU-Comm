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
    expect(rows.get("https://a.example text 1")).toEqual({ key: "https://a.example text 1", at: 5, file: f, spoiler: true, nonce: expect.any(String) });
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

describe("pendingFiles.send", () => {
  const later = () => {
    let settle!: (error?: string) => void;
    const done = new Promise<void>((resolve, reject) => (settle = (error) => (error === undefined ? resolve() : reject(new Error(error)))));
    return { run: () => done, settle };
  };

  it("marks the file as sending, then clears it once the send is confirmed", async () => {
    const { rows, store } = memoryStore();
    const files = createPendingFiles(store, "o", () => 1);
    const f = file();
    files.set("c", { file: f, spoiler: false }, 100);
    let changes = 0;
    files.watch(() => changes++);
    const send = later();
    const sent = files.send("c", f, send.run);
    expect(files.status("c", f)).toEqual({ sending: true });
    expect(changes).toBe(1);
    send.settle();
    expect(await sent).toBe(true);
    expect(files.status("c", f)).toBeUndefined();
    expect(files.holds("c", f)).toBe(false);
    expect(changes).toBe(2);
    await files.settled();
    expect(rows.size).toBe(0);
  });

  it("keeps the file with the error when the send fails", async () => {
    const files = createPendingFiles(null, "o");
    const f = file();
    files.set("c", { file: f, spoiler: true }, 100);
    const send = later();
    const sent = files.send("c", f, send.run);
    send.settle("Not connected");
    expect(await sent).toBe(false);
    expect(files.status("c", f)).toEqual({ sending: false, error: "Not connected" });
    expect(await files.get("c")).toEqual({ file: f, spoiler: true });
  });

  it("does not send the same file twice while it is sending (a new pane after a reconnect)", async () => {
    const files = createPendingFiles(null, "o");
    const f = file();
    files.set("c", { file: f, spoiler: false }, 100);
    const first = later();
    const sent = files.send("c", f, first.run);
    let again = 0;
    expect(await files.send("c", f, async () => void again++)).toBe(false);
    expect(again).toBe(0);
    expect(files.status("c", f)).toEqual({ sending: true });
    first.settle();
    await sent;
    expect(files.holds("c", f)).toBe(false);
  });

  it("a send again clears the old error; another file has no status", async () => {
    const files = createPendingFiles(null, "o");
    const f = file();
    files.set("c", { file: f, spoiler: false }, 100);
    await files.send("c", f, () => Promise.reject(new Error("no")));
    expect(files.status("c", file())).toBeUndefined();
    void files.send("c", f, later().run);
    expect(files.status("c", f)).toEqual({ sending: true });
  });

  it("an old send that ends leaves a file picked meanwhile alone", async () => {
    const files = createPendingFiles(null, "o");
    const [f, g] = [file("a.png"), file("b.png")];
    files.set("c", { file: f, spoiler: false }, 100);
    const send = later();
    const sent = files.send("c", f, send.run);
    files.set("c", { file: g, spoiler: false }, 100);
    send.settle();
    await sent;
    expect(files.holds("c", g)).toBe(true);
  });

  it("discarding the file drops its error; the watch can end", async () => {
    const files = createPendingFiles(null, "o");
    const f = file();
    files.set("c", { file: f, spoiler: false }, 100);
    let changes = 0;
    const stop = files.watch(() => changes++);
    await files.send("c", f, () => Promise.reject(new Error("no")));
    files.clear("c");
    expect(files.status("c", f)).toBeUndefined();
    stop();
    await files.send("c", f, async () => {});
    expect(changes).toBe(3);
  });

  it("sends each retry with the same nonce, also after a reload; another file gets a new one", async () => {
    const { store } = memoryStore();
    let n = 0;
    const files = createPendingFiles(store, "o", () => 1, () => `n${++n}`);
    const f = file();
    files.set("c", { file: f, spoiler: false }, 100);
    files.set("c", { file: f, spoiler: true }, 100); // the spoiler changed
    const used: string[] = [];
    const fail = async (nonce: string) => {
      used.push(nonce);
      throw new Error("no");
    };
    await files.send("c", f, fail);
    await files.send("c", f, fail);
    await files.settled();
    const after = createPendingFiles(store, "o", () => 2, () => "fresh");
    const back = (await after.get("c"))!;
    await after.send("c", back.file, fail);
    files.set("c", { file: file("b.png"), spoiler: false }, 100);
    await files.send("c", (await files.get("c"))!.file, fail);
    expect(used).toEqual(["n1", "n1", "n1", "n2"]);
  });
});
