// Custom emoji show at most 48 px (jumbo emoji): 96 px is enough for
// retina screens. Still images are scaled down before the upload, an
// optional square crop first; animations are kept (a canvas has only one
// frame).
export const EMOJI_SIZE = 96;
// The smallest crop is the image's short side / MAX_ZOOM.
export const MAX_ZOOM = 8;

export type Size = { width: number; height: number };
// A square in image pixels.
export type Crop = { x: number; y: number; size: number };

const clamp = (v: number, lo: number, hi: number) => Math.min(Math.max(v, lo), hi);

// Fits inside EMOJI_SIZE, keeping the aspect; never larger.
export const fitSize = (width: number, height: number): Size => {
  const scale = Math.min(1, EMOJI_SIZE / width, EMOJI_SIZE / height);
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
};

// One large reduction looks rough even with "high" smoothing: halve while
// more than twice too large, then scale to the target.
export const downscaleSteps = (from: Size, to: Size): Size[] => {
  const steps: Size[] = [];
  let { width, height } = from;
  while (width / 2 > to.width && height / 2 > to.height) {
    width = Math.round(width / 2);
    height = Math.round(height / 2);
    steps.push({ width, height });
  }
  return [...steps, to];
};

// The image is scaled once for cropping: enough pixels for a full-size
// emoji at the most zoom.
export const workingScale = (width: number, height: number) => Math.min(1, (EMOJI_SIZE * MAX_ZOOM) / Math.min(width, height));

export const middleCrop = (width: number, height: number): Crop => {
  const size = Math.min(width, height);
  return { x: (width - size) / 2, y: (height - size) / 2, size };
};

export const moveCrop = (c: Crop, dx: number, dy: number, width: number, height: number): Crop => ({
  x: clamp(c.x + dx, 0, width - c.size),
  y: clamp(c.y + dy, 0, height - c.size),
  size: c.size,
});

// factor > 1 zooms in (a smaller square), around the crop's middle.
export const zoomCrop = (c: Crop, factor: number, width: number, height: number): Crop => {
  const short = Math.min(width, height);
  const size = clamp(c.size / factor, short / MAX_ZOOM, short);
  const half = (c.size - size) / 2;
  return moveCrop({ x: c.x, y: c.y, size }, half, half, width, height);
};

export const cropZoom = (c: Crop, width: number, height: number) => Math.min(width, height) / c.size;

const MOVES: Record<string, [number, number]> = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] };

// Arrow keys move by 5% of the crop (shift: 25%); + and - zoom.
export const cropKey = (c: Crop, key: string, shift: boolean, width: number, height: number): Crop | null => {
  const move = MOVES[key];
  if (move) {
    const step = c.size * (shift ? 0.25 : 0.05);
    return moveCrop(c, move[0] * step, move[1] * step, width, height);
  }
  if (key === "+" || key === "=") return zoomCrop(c, 1.25, width, height);
  if (key === "-" || key === "_") return zoomCrop(c, 0.8, width, height);
  return null;
};

// Animations, and small images that are not cropped, are uploaded as
// they are (the limit is still checked).
export const keepsOriginal = (s: Size & { animated: boolean; cropping: boolean }) =>
  s.animated || (!s.cropping && s.width <= EMOJI_SIZE && s.height <= EMOJI_SIZE);

const ascii = (b: Uint8Array, at: number, n: number) => String.fromCharCode(...b.subarray(at, at + n));

// More than one frame, or a loop extension. Walks the blocks: image data
// may contain any byte.
const gifAnimated = (b: Uint8Array) => {
  if (b.length < 13) return false;
  let i = 13;
  if (b[10]! & 0x80) i += 3 << ((b[10]! & 7) + 1);
  const skipSubBlocks = () => {
    while (i < b.length && b[i] !== 0) i += b[i]! + 1;
    i++;
  };
  let frames = 0;
  while (i < b.length) {
    if (b[i] === 0x2c) {
      if (++frames > 1) return true;
      const flags = b[i + 9] ?? 0;
      i += 10;
      if (flags & 0x80) i += 3 << ((flags & 7) + 1);
      i++; // LZW code size
      skipSubBlocks();
    } else if (b[i] === 0x21) {
      if (b[i + 1] === 0xff && /^(NETSCAPE2\.0|ANIMEXTS1\.0)$/.test(ascii(b, i + 3, 11))) return true;
      i += 2;
      skipSubBlocks();
    } else return false;
  }
  return false;
};

const webpAnimated = (b: Uint8Array) => {
  const view = new DataView(b.buffer, b.byteOffset, b.byteLength);
  for (let i = 12; i + 8 <= b.length; ) {
    if (ascii(b, i, 4) === "ANIM") return true;
    const len = view.getUint32(i + 4, true);
    i += 8 + len + (len & 1);
  }
  return false;
};

// APNG: an acTL chunk before the image data.
const pngAnimated = (b: Uint8Array) => {
  const view = new DataView(b.buffer, b.byteOffset, b.byteLength);
  for (let i = 8; i + 8 <= b.length; ) {
    const type = ascii(b, i + 4, 4);
    if (type === "acTL") return true;
    if (type === "IDAT") return false;
    i += 12 + view.getUint32(i, false);
  }
  return false;
};

// From the file's bytes; false for anything it does not know.
export const isAnimated = (b: Uint8Array) => {
  if (ascii(b, 0, 4) === "GIF8") return gifAnimated(b);
  if (ascii(b, 0, 4) === "RIFF" && ascii(b, 8, 4) === "WEBP") return webpAnimated(b);
  if (ascii(b, 1, 3) === "PNG") return pngAnimated(b);
  return false;
};
