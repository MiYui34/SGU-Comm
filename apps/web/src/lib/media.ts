// Inline audio and video attachments.

export type MediaKind = "audio" | "video";

// In step with the server's files/media.go.
const AUDIO = /\.(mp3|m4a|ogg|oga|opus|wav|flac)$/i;
const VIDEO = /\.(mp4|m4v|webm)$/i;

// By type; uploads from before the server kept media types are downloads,
// so their name decides (the server serves them with the right type).
export const mediaKind = (u: { type: string; name: string }): MediaKind | null => {
  if (u.type.startsWith("audio/")) return "audio";
  if (u.type.startsWith("video/")) return "video";
  if (u.type !== "application/octet-stream") return null;
  return AUDIO.test(u.name) ? "audio" : VIDEO.test(u.name) ? "video" : null;
};

export const formatDuration = (seconds: number) => {
  const s = Number.isFinite(seconds) ? Math.floor(seconds) : 0;
  const mm = String(Math.floor(s / 60) % 60);
  const ss = String(s % 60).padStart(2, "0");
  return s >= 3600 ? `${Math.floor(s / 3600)}:${mm.padStart(2, "0")}:${ss}` : `${mm}:${ss}`;
};

// Seconds for display and seeking: recordings without a length (WebM from
// MediaRecorder) report Infinity.
export const finite = (seconds: number) => (Number.isFinite(seconds) ? seconds : 0);

// One plays at a time: starting one pauses the one before.
let playing: HTMLMediaElement | null = null;
export const onlyOne = (el: HTMLMediaElement) => {
  if (playing && playing !== el) playing.pause();
  playing = el;
};

// Spoiler attachments are marked by name, as in Discord (no extra field on
// the server); they show without the prefix, hidden until clicked.
const SPOILER_PREFIX = "SPOILER_";

export const asSpoiler = (file: File, on: boolean): File =>
  on && !file.name.startsWith(SPOILER_PREFIX) ? new File([file], SPOILER_PREFIX + file.name, { type: file.type }) : file;

export const spoilerUpload = <T extends { name: string }>(u: T): T & { spoiler?: true } =>
  u.name.startsWith(SPOILER_PREFIX) ? { ...u, name: u.name.slice(SPOILER_PREFIX.length), spoiler: true } : u;
