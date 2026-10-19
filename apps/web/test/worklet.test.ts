import { describe, expect, it, vi } from "vitest";
import { loadCaptureWorklet, startAfter } from "../src/lib/voice/worklet";

const flush = () => new Promise((r) => setTimeout(r));

describe("loadCaptureWorklet", () => {
  it("adds the module once per context", async () => {
    const addModule = vi.fn(async () => {});
    const ctx = { audioWorklet: { addModule } } as unknown as AudioContext;
    await Promise.all([loadCaptureWorklet(ctx), loadCaptureWorklet(ctx)]);
    expect(addModule).toHaveBeenCalledTimes(1);
  });
  it("tries again after a failed load", async () => {
    const addModule = vi.fn().mockRejectedValueOnce(new Error("offline")).mockResolvedValue(undefined);
    const ctx = { audioWorklet: { addModule } } as unknown as AudioContext;
    await expect(loadCaptureWorklet(ctx)).rejects.toThrow("offline");
    await loadCaptureWorklet(ctx);
    expect(addModule).toHaveBeenCalledTimes(2);
  });
});

describe("startAfter", () => {
  const fakeContext = () => {
    const calls: string[] = [];
    return { calls, suspend: vi.fn(async () => void calls.push("suspend")), resume: vi.fn(async () => void calls.push("resume")) };
  };
  it("holds the output until the module is there", async () => {
    const ctx = fakeContext();
    let done!: () => void;
    startAfter(ctx, new Promise<void>((r) => (done = r)));
    expect(ctx.suspend).toHaveBeenCalledTimes(1); // at once, before the output starts
    await flush();
    expect(ctx.resume).not.toHaveBeenCalled();
    done();
    await flush();
    expect(ctx.calls).toEqual(["suspend", "resume"]);
  });
  it("starts the output also when the module fails", async () => {
    const ctx = fakeContext();
    startAfter(ctx, Promise.reject(new Error("no worklet")));
    await flush();
    expect(ctx.calls).toEqual(["suspend", "resume"]);
  });
  it("ignores a context that was closed meanwhile", async () => {
    const ctx = { suspend: vi.fn(async () => {}), resume: vi.fn(() => Promise.reject(new DOMException("closed", "InvalidStateError"))) };
    const unhandled = vi.fn();
    process.on("unhandledRejection", unhandled);
    startAfter(ctx, Promise.resolve());
    await flush();
    process.off("unhandledRejection", unhandled);
    expect(ctx.resume).toHaveBeenCalled();
    expect(unhandled).not.toHaveBeenCalled();
  });
});
