import { downscaleSteps, fitSize, workingScale, type Crop, type Size } from "./emojiResize";

// The canvas side of emoji resizing (browser only; jsdom has no canvas,
// so no tests: the math is in emojiResize).
type Rect = Size & { x: number; y: number };

const draw = (src: CanvasImageSource, r: Rect, to: Size) => {
  const c = document.createElement("canvas");
  c.width = to.width;
  c.height = to.height;
  const ctx = c.getContext("2d")!;
  ctx.imageSmoothingQuality = "high";
  ctx.drawImage(src, r.x, r.y, r.width, r.height, 0, 0, to.width, to.height);
  return c;
};

const scaleDown = (src: CanvasImageSource, r: Rect, to: Size) => {
  let c: CanvasImageSource = src;
  for (const step of downscaleSteps(r, to)) {
    c = draw(c, r, step);
    r = { x: 0, y: 0, ...step };
  }
  return c as HTMLCanvasElement;
};

// The image, scaled once for cropping (see workingScale); original: its
// size as it came.
export const loadEmojiImage = async (file: Blob) => {
  const bitmap = await createImageBitmap(file);
  const original = { width: bitmap.width, height: bitmap.height };
  const s = workingScale(original.width, original.height);
  const to = { width: Math.round(original.width * s), height: Math.round(original.height * s) };
  const image = scaleDown(bitmap, { x: 0, y: 0, ...original }, to);
  bitmap.close();
  return { image, original };
};

// The emoji: the crop (or the whole image) fit inside 96×96.
export const renderEmoji = (image: HTMLCanvasElement, crop: Crop | null) => {
  const r = crop ? { x: crop.x, y: crop.y, width: crop.size, height: crop.size } : { x: 0, y: 0, width: image.width, height: image.height };
  return scaleDown(image, r, fitSize(r.width, r.height));
};

// PNG, or WebP if the browser has it and it is smaller; both keep
// transparency (quality 1: lossless WebP in Chromium, lossy in Firefox).
export const encodeEmoji = async (c: HTMLCanvasElement): Promise<Blob> => {
  const blob = (type: string) => new Promise<Blob | null>((resolve) => c.toBlob(resolve, type, 1));
  const [png, webp] = await Promise.all([blob("image/png"), blob("image/webp")]);
  if (!png) throw new Error("cannot encode the image");
  // Without WebP, toBlob gives a PNG.
  return webp?.type === "image/webp" && webp.size < png.size ? webp : png;
};
