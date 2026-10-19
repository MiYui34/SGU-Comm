import { describe, expect, it } from "vitest";
import {
  EMOJI_SIZE,
  MAX_ZOOM,
  cropKey,
  cropZoom,
  downscaleSteps,
  fitSize,
  isAnimated,
  keepsOriginal,
  middleCrop,
  moveCrop,
  workingScale,
  zoomCrop,
} from "../src/lib/emojiResize";

describe("fitSize", () => {
  it("scales down to fit inside 96, keeping the aspect", () => {
    expect(fitSize(1920, 1080)).toEqual({ width: 96, height: 54 });
    expect(fitSize(300, 600)).toEqual({ width: 48, height: 96 });
  });

  it("never scales up", () => {
    expect(fitSize(40, 20)).toEqual({ width: 40, height: 20 });
    expect(fitSize(96, 96)).toEqual({ width: 96, height: 96 });
  });

  it("keeps at least one pixel", () => {
    expect(fitSize(10_000, 10)).toEqual({ width: 96, height: 1 });
  });

  it("rounds fractional sizes (crops)", () => {
    expect(fitSize(50.4, 50.4)).toEqual({ width: 50, height: 50 });
  });
});

describe("downscaleSteps", () => {
  it("halves while more than twice too large, then goes to the target", () => {
    expect(downscaleSteps({ width: 1000, height: 800 }, { width: 96, height: 77 })).toEqual([
      { width: 500, height: 400 },
      { width: 250, height: 200 },
      { width: 125, height: 100 },
      { width: 96, height: 77 },
    ]);
  });

  it("is one step for small reductions", () => {
    expect(downscaleSteps({ width: 150, height: 150 }, { width: 96, height: 96 })).toEqual([{ width: 96, height: 96 }]);
  });
});

describe("workingScale", () => {
  it("keeps enough pixels for the most zoom", () => {
    expect(workingScale(4000, 3000)).toBe((EMOJI_SIZE * MAX_ZOOM) / 3000);
    expect(workingScale(500, 400)).toBe(1);
  });
});

describe("crop", () => {
  const W = 400;
  const H = 200;

  it("starts as the largest square in the middle", () => {
    expect(middleCrop(W, H)).toEqual({ x: 100, y: 0, size: 200 });
  });

  it("moves inside the image", () => {
    const c = { x: 100, y: 0, size: 100 };
    expect(moveCrop(c, 50, 30, W, H)).toEqual({ x: 150, y: 30, size: 100 });
    expect(moveCrop(c, -500, -10, W, H)).toEqual({ x: 0, y: 0, size: 100 });
    expect(moveCrop(c, 500, 500, W, H)).toEqual({ x: 300, y: 100, size: 100 });
  });

  it("zooms around the middle of the crop", () => {
    expect(zoomCrop({ x: 100, y: 0, size: 200 }, 2, W, H)).toEqual({ x: 150, y: 50, size: 100 });
  });

  it("keeps the zoom between 1 and the most", () => {
    expect(zoomCrop({ x: 100, y: 0, size: 200 }, 0.5, W, H).size).toBe(200);
    expect(zoomCrop({ x: 100, y: 0, size: 200 }, 100, W, H).size).toBe(200 / MAX_ZOOM);
  });

  it("stays inside when zooming out near an edge", () => {
    expect(zoomCrop({ x: 300, y: 100, size: 100 }, 0.5, W, H)).toEqual({ x: 200, y: 0, size: 200 });
  });

  it("gives the zoom of a crop", () => {
    expect(cropZoom({ x: 0, y: 0, size: 50 }, W, H)).toBe(4);
  });
});

describe("cropKey", () => {
  const c = { x: 100, y: 50, size: 100 };

  it("moves with the arrow keys (shift: more)", () => {
    expect(cropKey(c, "ArrowRight", false, 400, 200)).toEqual({ x: 105, y: 50, size: 100 });
    expect(cropKey(c, "ArrowUp", false, 400, 200)).toEqual({ x: 100, y: 45, size: 100 });
    expect(cropKey(c, "ArrowLeft", true, 400, 200)).toEqual({ x: 75, y: 50, size: 100 });
    expect(cropKey(c, "ArrowDown", true, 400, 200)).toEqual({ x: 100, y: 75, size: 100 });
  });

  it("zooms with + and -", () => {
    expect(cropKey(c, "+", false, 400, 200)!.size).toBe(80);
    expect(cropKey(c, "=", false, 400, 200)!.size).toBe(80);
    expect(cropKey(c, "-", false, 400, 200)!.size).toBe(125);
  });

  it("ignores other keys", () => {
    expect(cropKey(c, "a", false, 400, 200)).toBeNull();
  });
});

describe("keepsOriginal", () => {
  it("keeps animations and small images that are not cropped", () => {
    expect(keepsOriginal({ animated: true, width: 500, height: 500, cropping: true })).toBe(true);
    expect(keepsOriginal({ animated: false, width: 96, height: 64, cropping: false })).toBe(true);
  });

  it("resizes large or cropped still images", () => {
    expect(keepsOriginal({ animated: false, width: 97, height: 64, cropping: false })).toBe(false);
    expect(keepsOriginal({ animated: false, width: 64, height: 32, cropping: true })).toBe(false);
  });
});

// Minimal files: only the structure the check reads.
const bytes = (...parts: (string | number[])[]) =>
  new Uint8Array(parts.flatMap((p) => (typeof p === "string" ? [...p].map((c) => c.charCodeAt(0)) : p)));
const u32 = (n: number) => [n & 255, (n >> 8) & 255, (n >> 16) & 255, n >>> 24];

// A 1×1 GIF frame: descriptor, LZW code size, one data sub-block, end.
const gifFrame = [0x2c, 0, 0, 0, 0, 1, 0, 1, 0, 0, 2, 2, 0x44, 0x01, 0];
const gif = (...body: number[][]) =>
  bytes("GIF89a", [1, 0, 1, 0, 0x80, 0, 0], [0, 0, 0, 255, 255, 255], ...body, [0x3b]);
const gce = [0x21, 0xf9, 4, 0, 10, 0, 0, 0];
const netscape = [0x21, 0xff, 11, ...bytes("NETSCAPE2.0"), 3, 1, 0, 0, 0];

const webp = (...chunks: [string, number[]][]) => {
  const body = chunks.flatMap(([id, data]) => [...bytes(id), ...u32(data.length), ...data, ...(data.length % 2 ? [0] : [])]);
  return bytes("RIFF", u32(4 + body.length), "WEBP", body);
};

describe("isAnimated", () => {
  it("finds GIFs with more than one frame", () => {
    expect(isAnimated(gif(gce, gifFrame, gce, gifFrame))).toBe(true);
    expect(isAnimated(gif(gce, gifFrame))).toBe(false);
  });

  it("finds GIFs with a loop extension", () => {
    expect(isAnimated(gif(netscape, gifFrame))).toBe(true);
  });

  it("skips a 0x2C byte inside image data", () => {
    const frame = [0x2c, 0, 0, 0, 0, 1, 0, 1, 0, 0, 2, 2, 0x2c, 0x2c, 0];
    expect(isAnimated(gif(frame))).toBe(false);
  });

  it("finds WebPs with an ANIM chunk", () => {
    expect(isAnimated(webp(["VP8X", [2, 0, 0, 0, 0, 0, 0, 0, 0, 0]], ["ANIM", [0, 0, 0, 0, 0, 0]]))).toBe(true);
    expect(isAnimated(webp(["VP8L", [0x2f, 0, 0, 0, 0]]))).toBe(false);
  });

  it("finds animated PNGs (acTL before IDAT)", () => {
    const be = (n: number) => u32(n).reverse();
    const chunk = (id: string, data: number[]) => [...be(data.length), ...bytes(id), ...data, 0, 0, 0, 0];
    const png = (...chunks: number[][]) => bytes([0x89], "PNG", [13, 10, 26, 10], ...chunks);
    const ihdr = chunk("IHDR", Array(13).fill(0));
    expect(isAnimated(png(ihdr, chunk("acTL", Array(8).fill(0)), chunk("IDAT", [0])))).toBe(true);
    expect(isAnimated(png(ihdr, chunk("IDAT", [0]), chunk("acTL", Array(8).fill(0))))).toBe(false);
  });

  it("is false for other or broken files", () => {
    expect(isAnimated(bytes([0x89], "PNG"))).toBe(false);
    expect(isAnimated(bytes("GIF89a", [1, 0]))).toBe(false);
    expect(isAnimated(bytes("RIFF", u32(100), "WEBP", "AN"))).toBe(false);
  });
});
