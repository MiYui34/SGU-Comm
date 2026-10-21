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

// How an attachment shows: a picture, a player, or a file card (also a
// picture or player the browser could not load: AVIF in an old WebView).
export const attachmentView = (u: { type: string; name: string }, failed: boolean): "image" | MediaKind | "file" =>
  failed ? "file" : u.type.startsWith("image/") ? "image" : (mediaKind(u) ?? "file");

// The source to try after a picture failed to load from src: the picture
// after its preview; null: none, show the file card.
export const afterImageError = (src: string | null, u: { url: string }) => (src !== null && src !== u.url ? u.url : null);

export const formatDuration = (seconds: number) => {
  const s = Number.isFinite(seconds) ? Math.floor(seconds) : 0;
  const mm = String(Math.floor(s / 60) % 60);
  const ss = String(s % 60).padStart(2, "0");
  return s >= 3600 ? `${Math.floor(s / 3600)}:${mm.padStart(2, "0")}:${ss}` : `${mm}:${ss}`;
};

// Seconds for display and seeking: recordings without a length (WebM from
// MediaRecorder) report Infinity.
export const finite = (seconds: number) => (Number.isFinite(seconds) ? seconds : 0);

type Sized = { width?: number; height?: number };

// The box a picture fills before it loads (no jump when it does): its size
// in CSS pixels, scaled down into max as the chat's CSS does. null: unknown.
export const imageBox = ({ width, height }: Sized, max = { width: 400, height: 300 }) => {
  if (!width || !height) return null;
  const scale = Math.min(1, max.width / width, max.height / height);
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
};

// The server's smaller picture (made with ffmpeg, if it has it): a large
// picture's in the chat (the original opens in the lightbox), a video's
// poster.
export const chatSrc = (u: { url: string; preview?: string }) => u.preview ?? u.url;

// A video with a poster loads nothing until played; without one its
// metadata and first frame.
export const videoPreload = (poster: string | undefined) => (poster ? "none" : "metadata");

// CSS aspect-ratio of a video (its box width is fixed).
export const mediaRatio = ({ width, height }: Sized) => (width && height ? `${width} / ${height}` : undefined);

// Seconds to show: the player's, else the server's (milliseconds, known
// before the metadata loads).
export const shownLength = (seconds: number, knownMs = 0) => finite(seconds) || knownMs / 1000;

// One plays at a time: starting one pauses the one before.
let playing: HTMLMediaElement | null = null;
export const onlyOne = (el: HTMLMediaElement) => {
  if (playing && playing !== el) playing.pause();
  playing = el;
};

// A player that leaves the screen (closed, or the client re-mounted on a
// reconnect) stops and drops its source: else the browser keeps it as the
// media-session target, and a media key or a headset resumes it hours later.
export const releaseMedia = (el: HTMLMediaElement) => {
  el.pause();
  el.removeAttribute("src");
  el.load();
  if (playing === el) playing = null;
};

// As a Svelte attachment: {@attach releaseOnDestroy}.
export const releaseOnDestroy = (el: HTMLMediaElement) => () => releaseMedia(el);

// Spoiler attachments are marked by name, as in Discord (no extra field on
// the server); they show without the prefix, hidden until clicked.
const SPOILER_PREFIX = "SPOILER_";

export const asSpoiler = (file: File, on: boolean): File =>
  on && !file.name.startsWith(SPOILER_PREFIX) ? new File([file], SPOILER_PREFIX + file.name, { type: file.type }) : file;

export const spoilerUpload = <T extends { name: string }>(u: T): T & { spoiler?: true } =>
  u.name.startsWith(SPOILER_PREFIX) ? { ...u, name: u.name.slice(SPOILER_PREFIX.length), spoiler: true } : u;

// A download button's link: the server sends the file as an attachment
// (media are served inline, so a plain link would play it).
export const downloadUrl = (url: string) => `${url}${url.includes("?") ? "&" : "?"}download`;
