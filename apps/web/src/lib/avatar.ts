import { missingTextures, type ServerState } from "@kallisti/protocol";
import { getContext, setContext } from "svelte";

const COLORS = ["#e2745b", "#d9a33b", "#5aa469", "#3ba99c", "#4f8fd6", "#8a6fd1", "#c763a6", "#7c8794"];

export const initials = (name: string) => [...name.trim()][0]?.toUpperCase() ?? "?";

// Small string hash (FNV-1a) so a name always gets the same color.
export const avatarColor = (name: string) => {
  let hash = 0x811c9dc5;
  for (const ch of name) hash = Math.imul(hash ^ ch.codePointAt(0)!, 0x01000193);
  return COLORS[(hash >>> 0) % COLORS.length]!;
};

// Avatar pictures (Mumble textures): only common image types are shown,
// by their first bytes (not SVG, not old Mumble raw textures).
const ascii = (b: Uint8Array, from: number, to: number) => String.fromCharCode(...b.subarray(from, to));
const SIGNATURES: [string, (b: Uint8Array) => boolean][] = [
  ["image/png", (b) => [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a].every((x, i) => b[i] === x)],
  ["image/jpeg", (b) => b[0] === 0xff && b[1] === 0xd8 && b[2] === 0xff],
  ["image/gif", (b) => ascii(b, 0, 4) === "GIF8"],
  ["image/webp", (b) => ascii(b, 0, 4) === "RIFF" && ascii(b, 8, 12) === "WEBP"],
];

export const imageType = (b: Uint8Array): string | null => SIGNATURES.find(([, ok]) => ok(b))?.[0] ?? null;

// A picture's size from its header (null: unknown). Each viewer decodes
// pictures in full, so very large ones are not shown.
type Size = { width: number; height: number };
const u16 = (b: Uint8Array, i: number, little = false) => (little ? b[i]! | (b[i + 1]! << 8) : (b[i]! << 8) | b[i + 1]!);
const u24le = (b: Uint8Array, i: number) => b[i]! | (b[i + 1]! << 8) | (b[i + 2]! << 16);
const jpegSize = (b: Uint8Array): Size | null => {
  for (let i = 2; i + 9 < b.length; ) {
    if (b[i] !== 0xff) return null;
    const marker = b[i + 1]!;
    if (marker === 0xff) {
      i++; // padding
      continue;
    }
    // Start of frame (not DHT, JPG or DAC, which share the range).
    if (marker >= 0xc0 && marker <= 0xcf && ![0xc4, 0xc8, 0xcc].includes(marker))
      return { height: u16(b, i + 5), width: u16(b, i + 7) };
    i += 2 + u16(b, i + 2);
  }
  return null;
};
export const imageSize = (b: Uint8Array): Size | null => {
  switch (imageType(b)) {
    case "image/png":
      return b.length >= 24 ? { width: u16(b, 16) * 65536 + u16(b, 18), height: u16(b, 20) * 65536 + u16(b, 22) } : null;
    case "image/gif":
      return b.length >= 10 ? { width: u16(b, 6, true), height: u16(b, 8, true) } : null;
    case "image/jpeg":
      return jpegSize(b);
    case "image/webp": {
      const chunk = ascii(b, 12, 16);
      if (chunk === "VP8 " && b.length >= 30) return { width: u16(b, 26, true) & 0x3fff, height: u16(b, 28, true) & 0x3fff };
      if (chunk === "VP8L" && b.length >= 25) {
        const bits = (b[21]! | (b[22]! << 8) | (b[23]! << 16) | (b[24]! << 24)) >>> 0;
        return { width: (bits & 0x3fff) + 1, height: ((bits >>> 14) & 0x3fff) + 1 };
      }
      if (chunk === "VP8X" && b.length >= 30) return { width: u24le(b, 24) + 1, height: u24le(b, 27) + 1 };
      return null;
    }
  }
  return null;
};
const MAX_SIDE = 1024;

// A data URL for a picture, made once per byte array (state keeps it).
const urls = new WeakMap<Uint8Array, string | null>();
export const pictureUrl = (b: Uint8Array | null): string | null => {
  if (!b) return null;
  if (!urls.has(b)) {
    const size = imageSize(b);
    const type = size && size.width <= MAX_SIDE && size.height <= MAX_SIDE ? imageType(b) : null;
    let bin = "";
    for (let i = 0; i < b.length; i += 0x8000) bin += String.fromCharCode(...b.subarray(i, i + 0x8000));
    urls.set(b, type && `data:${type};base64,${btoa(bin)}`);
  }
  return urls.get(b)!;
};

// The middle square of a picture, to crop it before scaling.
export const squareCrop = (width: number, height: number) => {
  const size = Math.min(width, height);
  return { x: (width - size) / 2, y: (height - size) / 2, size };
};

// Avatars find pictures by name: the client sets the lookup once (a
// context), so every Avatar shows the picture without extra props.
const KEY = Symbol("avatar pictures");
type Lookup = (name: string) => string | null;
export const setAvatarPictures = (lookup: Lookup) => setContext(KEY, lookup);
export const pictureFor = (): Lookup => getContext<Lookup | undefined>(KEY) ?? (() => null);

// Avatars known only by their hash, to fetch: once per user and picture.
// Users who left are forgotten (Murmur reuses session numbers).
export const texturesToFetch = (state: ServerState, asked: ReadonlySet<string>) => {
  const key = (session: number) => `${session}:${state.users[session]?.textureHash}`;
  const kept = [...asked].filter((k) => state.users[Number(k.split(":")[0])] !== undefined);
  const sessions = missingTextures(state).filter((s) => !asked.has(key(s)));
  return { sessions, asked: new Set([...kept, ...sessions.map(key)]) };
};
