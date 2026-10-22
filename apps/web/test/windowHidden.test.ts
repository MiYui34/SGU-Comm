import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { POLL_MS, watchWindowHidden, type WindowApi } from "../src/lib/windowHidden";

// A document with a settable visibility and focus.
const fakeDoc = (state: DocumentVisibilityState = "visible", focused = true) => {
  const target = new EventTarget();
  const doc = {
    visibilityState: state,
    focused,
    hasFocus: () => doc.focused,
    addEventListener: target.addEventListener.bind(target),
    removeEventListener: target.removeEventListener.bind(target),
    show: (s: DocumentVisibilityState) => {
      doc.visibilityState = s;
      target.dispatchEvent(new Event("visibilitychange"));
    },
  };
  return doc;
};

// The Tauri window: minimised or not, with its resize and focus events.
const fakeWindow = (minimized = false) => {
  const resized = new Set<() => void>();
  const focus = new Set<(e: { payload: boolean }) => void>();
  const win = {
    minimized,
    visible: true,
    calls: 0,
    isMinimized: async () => (win.calls++, win.minimized),
    isVisible: async () => win.visible,
    onResized: async (cb: () => void) => (resized.add(cb), () => void resized.delete(cb)),
    onFocusChanged: async (cb: (e: { payload: boolean }) => void) => (focus.add(cb), () => void focus.delete(cb)),
    resize: () => resized.forEach((cb) => cb()),
    focus: (on: boolean) => focus.forEach((cb) => cb({ payload: on })),
    listeners: () => resized.size + focus.size,
  };
  return win satisfies WindowApi;
};

const flush = () => vi.advanceTimersByTimeAsync(0);

describe("watchWindowHidden", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("follows the page's visibility", async () => {
    const doc = fakeDoc();
    const changes: boolean[] = [];
    const stop = watchWindowHidden((h) => changes.push(h), { doc });
    await flush();
    expect(changes).toEqual([]);
    doc.show("hidden");
    doc.show("hidden");
    doc.show("visible");
    expect(changes).toEqual([true, false]);
    stop();
    doc.show("hidden");
    expect(changes).toEqual([true, false]);
  });

  it("reports a page hidden at the start", () => {
    const changes: boolean[] = [];
    watchWindowHidden((h) => changes.push(h), { doc: fakeDoc("hidden") });
    expect(changes).toEqual([true]);
  });

  // WebKitGTK may keep a minimised window "visible".
  it("hidden while the desktop window is minimised (asked on blur and resize)", async () => {
    const doc = fakeDoc();
    const win = fakeWindow();
    const changes: boolean[] = [];
    watchWindowHidden((h) => changes.push(h), { doc, win });
    await flush();
    win.minimized = true;
    win.focus(false);
    await flush();
    expect(changes).toEqual([true]);
    win.minimized = false;
    win.resize();
    await flush();
    expect(changes).toEqual([true, false]);
  });

  it("hidden while the desktop window is not visible", async () => {
    const win = fakeWindow();
    const changes: boolean[] = [];
    watchWindowHidden((h) => changes.push(h), { doc: fakeDoc(), win });
    await flush();
    win.visible = false;
    win.resize();
    await flush();
    expect(changes).toEqual([true]);
  });

  // A focused window is not minimised: no need to ask.
  it("shown again when the window gets the focus", async () => {
    const win = fakeWindow(true);
    const changes: boolean[] = [];
    watchWindowHidden((h) => changes.push(h), { doc: fakeDoc("visible", false), win });
    await flush();
    expect(changes).toEqual([true]);
    win.focus(true);
    expect(changes).toEqual([true, false]);
  });

  // Not every platform sends an event on minimise or restore.
  it("asks again while the window has no focus, not while it has", async () => {
    const doc = fakeDoc("visible", false);
    const win = fakeWindow();
    const changes: boolean[] = [];
    const stop = watchWindowHidden((h) => changes.push(h), { doc, win });
    await flush();
    win.minimized = true;
    await vi.advanceTimersByTimeAsync(POLL_MS);
    expect(changes).toEqual([true]);
    win.minimized = false;
    await vi.advanceTimersByTimeAsync(POLL_MS);
    expect(changes).toEqual([true, false]);
    win.focus(true);
    const calls = win.calls;
    await vi.advanceTimersByTimeAsync(10 * POLL_MS);
    expect(win.calls).toBe(calls);
    win.focus(false);
    await vi.advanceTimersByTimeAsync(POLL_MS);
    expect(win.calls).toBeGreaterThan(calls);
    stop();
    const stopped = win.calls;
    await vi.advanceTimersByTimeAsync(10 * POLL_MS);
    expect(win.calls).toBe(stopped);
    expect(win.listeners()).toBe(0);
  });

  it("hidden page or minimised window: hidden until both are back", async () => {
    const doc = fakeDoc();
    const win = fakeWindow();
    const changes: boolean[] = [];
    watchWindowHidden((h) => changes.push(h), { doc, win });
    await flush();
    win.minimized = true;
    win.focus(false);
    await flush();
    doc.show("hidden");
    doc.show("visible");
    expect(changes).toEqual([true]);
    win.focus(true);
    expect(changes).toEqual([true, false]);
  });

  // An older answer from the window must not win over a newer one.
  it("keeps the newest answer", async () => {
    const win = fakeWindow();
    let answer: ((v: boolean) => void) | undefined;
    win.isMinimized = () => new Promise<boolean>((r) => (answer = r));
    const changes: boolean[] = [];
    watchWindowHidden((h) => changes.push(h), { doc: fakeDoc(), win });
    await flush();
    win.focus(false);
    const old = answer!;
    win.focus(true);
    old(true);
    await flush();
    expect(changes).toEqual([]);
  });

  it("stopped before the window's listeners are in: they are removed", async () => {
    const win = fakeWindow();
    const stop = watchWindowHidden(() => {}, { doc: fakeDoc(), win });
    stop();
    await flush();
    expect(win.listeners()).toBe(0);
  });

  it("an error from the window counts as shown", async () => {
    const win = fakeWindow(true);
    win.isMinimized = () => Promise.reject(new Error("no window"));
    const changes: boolean[] = [];
    watchWindowHidden((h) => changes.push(h), { doc: fakeDoc("visible", false), win });
    await flush();
    expect(changes).toEqual([]);
  });
});
