import { fitSize } from "./protocol";

// What a streamer sends: a screen share and a camera (at most one each),
// at a quality the streamer picks, also while live.

export type Kind = "screen" | "camera";
// height: of a landscape picture (the short side); fps: frames a second.
export type Quality = { height: number; fps: number };
// The server's limits (its "ready"): the upload of both streams together,
// bytes a second, and the largest side of a picture.
export type Limits = { bytesPerSecond: number; maxSide: number };

const HEIGHTS = [480, 720, 1080];
const FPS = [15, 30, 60];
export const DEFAULT_LIMITS: Limits = { bytesPerSecond: 1 << 20, maxSide: 4096 };
// A still screen wants sharp text; a face wants smooth motion.
export const DEFAULT_QUALITY: Record<Kind, Quality> = { screen: { height: 1080, fps: 15 }, camera: { height: 720, fps: 30 } };

// Of the upload: a part for sound and keyframes, the rest shared (fixed
// parts: a stream that starts does not change the other's encoder).
const HEADROOM = 0.8;
const SHARE: Record<Kind, number> = { screen: 0.75, camera: 0.25 };
const BITS_PER_PIXEL = 0.08; // per frame: 1080p at 15 fps is 2.5 Mbit/s
const MIN_BITRATE = 100_000;

export const parseLimits = (v: unknown): Limits => {
  const l = (v ?? {}) as Record<string, unknown>;
  const pos = (n: unknown, d: number) => (typeof n === "number" && n > 0 ? n : d);
  return { bytesPerSecond: pos(l.bytesPerSecond, DEFAULT_LIMITS.bytesPerSecond), maxSide: pos(l.maxSide, DEFAULT_LIMITS.maxSide) };
};

// The encoder's settings for a source picture at a quality: the source's
// aspect within 16:9 of the chosen height (turned for a portrait picture)
// and the server's largest side; a bitrate for its size and frame rate,
// within the stream's part of the upload.
export const encoding = (source: { width: number; height: number }, q: Quality, kind: Kind, limits: Limits) => {
  const [short, long] = [Math.min(q.height, limits.maxSide), Math.min(Math.round((q.height * 16) / 9), limits.maxSide)];
  const portrait = source.height > source.width;
  const size = fitSize(source.width, source.height, portrait ? short : long, portrait ? long : short);
  const budget = Math.floor(limits.bytesPerSecond * 8 * HEADROOM * SHARE[kind]);
  const bitrate = Math.max(MIN_BITRATE, Math.min(budget, Math.round(size.width * size.height * q.fps * BITS_PER_PIXEL)));
  return { ...size, framerate: q.fps, bitrate };
};

// A stored quality: one of the choices, else the default.
export const parseQuality = (v: unknown, kind: Kind): Quality => {
  const q = (v ?? {}) as Record<string, unknown>;
  return HEIGHTS.includes(q.height as number) && FPS.includes(q.fps as number) ? { height: q.height as number, fps: q.fps as number } : DEFAULT_QUALITY[kind];
};

export const qualityOptions = (limits: Limits) => ({ heights: HEIGHTS.filter((h) => h <= limits.maxSide), fps: FPS });

export const qualityLabel = (q: Quality) => `${q.height}p ${q.fps} fps`;
export const streamLabel = (name: string, camera: boolean) => `${name}'s ${camera ? "camera" : "screen"}`;

// What the browser can share (mobile browsers cannot share the screen).
export const sources = (env: { encode: boolean; display: boolean; camera: boolean; phone: boolean }): Record<Kind, boolean> => ({
  screen: env.encode && env.display && !env.phone,
  camera: env.encode && env.camera,
});

// The chosen camera (null: the default) at a quality.
export const cameraConstraints = (deviceId: string | null, q: Quality): MediaTrackConstraints => ({
  deviceId: deviceId ? { exact: deviceId } : undefined,
  width: { ideal: Math.round((q.height * 16) / 9) },
  height: { ideal: q.height },
  frameRate: { ideal: q.fps },
});

// The camera after the current one (a phone's front and back), or null.
export const nextCamera = (cameras: readonly { deviceId: string }[], current: string | null): string | null => {
  if (cameras.length < 2) return null;
  const i = cameras.findIndex((c) => c.deviceId === current);
  return cameras[(Math.max(0, i) + 1) % cameras.length]!.deviceId;
};
