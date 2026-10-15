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

// A data URL for a picture, made once per byte array (state keeps it).
const urls = new WeakMap<Uint8Array, string | null>();
export const pictureUrl = (b: Uint8Array | null): string | null => {
  if (!b) return null;
  if (!urls.has(b)) {
    const type = imageType(b);
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
