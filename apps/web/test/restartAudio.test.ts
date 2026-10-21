import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { documentPage, outputSignature, restartOnJoin, swapContext, watchOutputDevices } from "../src/lib/voice/restart";

type Device = Pick<MediaDeviceInfo, "kind" | "deviceId" | "groupId" | "label">;
const out = (deviceId: string, label: string, groupId = label): Device => ({ kind: "audiooutput", deviceId, groupId, label });
const mic = (deviceId: string, label: string): Device => ({ kind: "audioinput", deviceId, groupId: label, label });

describe("outputSignature", () => {
  it("changes when the default output changes (its label and group)", () => {
    const before = [out("default", "Default - AirPods", "g1"), out("a", "AirPods", "g1"), out("b", "MacBook Speakers", "g2")];
    const after = [out("default", "Default - MacBook Speakers", "g2"), out("b", "MacBook Speakers", "g2")];
    expect(outputSignature(after)).not.toBe(outputSignature(before));
  });
  it("ignores microphones and the order of the list", () => {
    const a = [out("a", "A"), out("b", "B"), mic("m", "Mic")];
    const b = [mic("n", "Other mic"), out("b", "B"), out("a", "A")];
    expect(outputSignature(a)).toBe(outputSignature(b));
  });
});

describe("restartOnJoin", () => {
  it("restarts when the user enters voice from the lobby", () => {
    expect(restartOnJoin({ inVoice: false, lobby: true, target: 3 })).toBe(true);
  });
  it("does not restart on a move between voice channels or into the lobby", () => {
    expect(restartOnJoin({ inVoice: true, lobby: true, target: 3 })).toBe(false);
    expect(restartOnJoin({ inVoice: false, lobby: true, target: 0 })).toBe(false);
  });
});

describe("watchOutputDevices", () => {
  const fakeMedia = (first: Device[]) => {
    let devices = first;
    const listeners = new Set<() => void>();
    return {
      set: (d: Device[]) => (devices = d),
      fire: () => listeners.forEach((l) => l()),
      listeners,
      media: {
        enumerateDevices: vi.fn(async () => devices as MediaDeviceInfo[]),
        addEventListener: (_: string, l: () => void) => listeners.add(l),
        removeEventListener: (_: string, l: () => void) => listeners.delete(l),
      },
    };
  };
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  // A page that can hide (a locked phone): the test shows it again.
  const fakePage = () => {
    let visible = true;
    const shown = new Set<() => void>();
    return {
      hide: () => (visible = false),
      show: () => ((visible = true), shown.forEach((l) => l())),
      shown,
      page: { visible: () => visible, onVisible: (l: () => void) => (shown.add(l), () => void shown.delete(l)) },
    };
  };

  it("waits while the page is hidden, and restarts when it shows again", async () => {
    const f = fakeMedia([out("a", "A")]);
    const p = fakePage();
    const changed = vi.fn();
    watchOutputDevices(f.media, changed, { debounceMs: 500, page: p.page });
    await vi.advanceTimersByTimeAsync(0);
    p.hide();
    f.set([out("a", "A"), out("h", "Headset")]);
    f.fire();
    await vi.advanceTimersByTimeAsync(1000);
    expect(changed).not.toHaveBeenCalled();
    p.show();
    await vi.advanceTimersByTimeAsync(0);
    expect(changed).toHaveBeenCalledTimes(1);
    p.hide();
    p.show(); // the same list: nothing more
    await vi.advanceTimersByTimeAsync(0);
    expect(changed).toHaveBeenCalledTimes(1);
  });

  it("compares with the list of the current context: a change undone while hidden does nothing", async () => {
    const f = fakeMedia([out("a", "A")]);
    const p = fakePage();
    const changed = vi.fn();
    watchOutputDevices(f.media, changed, { debounceMs: 500, page: p.page });
    await vi.advanceTimersByTimeAsync(0);
    p.hide();
    f.set([out("h", "Headset")]);
    f.fire();
    await vi.advanceTimersByTimeAsync(500);
    f.set([out("a", "A")]);
    f.fire();
    await vi.advanceTimersByTimeAsync(500);
    p.show();
    await vi.advanceTimersByTimeAsync(0);
    expect(changed).not.toHaveBeenCalled();
  });

  it("stops listening for the page too", async () => {
    const p = fakePage();
    const stop = watchOutputDevices(fakeMedia([out("a", "A")]).media, vi.fn(), { page: p.page });
    stop();
    expect(p.shown.size).toBe(0);
  });

  it("calls back once, after a burst of events, when the outputs changed", async () => {
    const f = fakeMedia([out("default", "Default - AirPods"), mic("m", "Mic")]);
    const changed = vi.fn();
    watchOutputDevices(f.media, changed, { debounceMs: 500 });
    await vi.advanceTimersByTimeAsync(0);
    f.set([out("default", "Default - Speakers"), mic("m", "Mic")]);
    f.fire();
    f.fire();
    await vi.advanceTimersByTimeAsync(400);
    f.fire();
    await vi.advanceTimersByTimeAsync(500);
    expect(changed).toHaveBeenCalledTimes(1);
    expect(f.media.enumerateDevices).toHaveBeenCalledTimes(2); // at the start, then once for the burst
  });

  it("does not call back when only the microphones changed, and compares with the last list", async () => {
    const f = fakeMedia([out("a", "A"), mic("m", "Mic")]);
    const changed = vi.fn();
    watchOutputDevices(f.media, changed, { debounceMs: 500 });
    await vi.advanceTimersByTimeAsync(0);
    f.set([out("a", "A"), mic("m", "Mic"), mic("n", "USB mic")]);
    f.fire();
    await vi.advanceTimersByTimeAsync(500);
    expect(changed).not.toHaveBeenCalled();
    f.set([out("b", "B")]);
    f.fire();
    await vi.advanceTimersByTimeAsync(500);
    f.fire(); // the same list again
    await vi.advanceTimersByTimeAsync(500);
    expect(changed).toHaveBeenCalledTimes(1);
  });

  it("uses only the newest list when reads overlap", async () => {
    const f = fakeMedia([out("a", "A")]);
    const changed = vi.fn();
    watchOutputDevices(f.media, changed, { debounceMs: 500 });
    await vi.advanceTimersByTimeAsync(0);
    let slow!: (d: MediaDeviceInfo[]) => void;
    f.media.enumerateDevices.mockImplementationOnce(() => new Promise((r) => (slow = r)));
    f.fire();
    await vi.advanceTimersByTimeAsync(500); // a slow read of the old list
    f.set([out("b", "B")]);
    f.fire();
    await vi.advanceTimersByTimeAsync(500); // the new list: one restart
    slow([out("a", "A")] as MediaDeviceInfo[]);
    await vi.advanceTimersByTimeAsync(0);
    expect(changed).toHaveBeenCalledTimes(1);
  });

  it("stops listening and drops a pending check", async () => {
    const f = fakeMedia([out("a", "A")]);
    const changed = vi.fn();
    const stop = watchOutputDevices(f.media, changed, { debounceMs: 500 });
    await vi.advanceTimersByTimeAsync(0);
    f.set([out("b", "B")]);
    f.fire();
    stop();
    await vi.advanceTimersByTimeAsync(1000);
    expect(changed).not.toHaveBeenCalled();
    expect(f.listeners.size).toBe(0);
  });

  it("does not call back for a list read after the stop", async () => {
    const f = fakeMedia([out("a", "A")]);
    let answer!: (d: MediaDeviceInfo[]) => void;
    const changed = vi.fn();
    const stop = watchOutputDevices(f.media, changed, { debounceMs: 500 });
    await vi.advanceTimersByTimeAsync(0);
    f.media.enumerateDevices.mockImplementationOnce(() => new Promise((r) => (answer = r)));
    f.fire();
    await vi.advanceTimersByTimeAsync(500);
    stop();
    answer([out("b", "B")] as MediaDeviceInfo[]);
    await vi.advanceTimersByTimeAsync(0);
    expect(changed).not.toHaveBeenCalled();
  });
});

describe("swapContext", () => {
  it("makes the new context, moves everything to it, then closes the old one", () => {
    const calls: string[] = [];
    const next = swapContext(
      "old",
      () => (calls.push("make"), "new"),
      (ctx) => calls.push(`move ${ctx}`),
      (ctx) => calls.push(`close ${ctx}`),
    );
    expect(next).toBe("new");
    expect(calls).toEqual(["make", "move new", "close old"]);
  });
  it("does nothing without a context (no voice support)", () => {
    const make = vi.fn();
    expect(swapContext(null, make, vi.fn(), vi.fn())).toBeNull();
    expect(make).not.toHaveBeenCalled();
  });
});

describe("documentPage", () => {
  it("calls back only when the document shows, until stopped", () => {
    const listeners = new Set<() => void>();
    const doc = {
      visibilityState: "hidden",
      addEventListener: (_: string, l: () => void) => listeners.add(l),
      removeEventListener: (_: string, l: () => void) => listeners.delete(l),
    };
    const page = documentPage(doc as unknown as Document);
    const shown = vi.fn();
    const stop = page.onVisible(shown);
    listeners.forEach((l) => l());
    expect(page.visible()).toBe(false);
    expect(shown).not.toHaveBeenCalled();
    doc.visibilityState = "visible";
    listeners.forEach((l) => l());
    expect(shown).toHaveBeenCalledTimes(1);
    stop();
    expect(listeners.size).toBe(0);
  });
});
