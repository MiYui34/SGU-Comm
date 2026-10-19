// Is a speaker sending music? Stereo or a high bitrate, as music mode
// sends (see opus.ts; our speech is 40 kbps mono). Read from the Opus TOC
// byte (RFC 6716, 3.1) and the packet sizes, so nothing is decoded.

export const isStereo = (opus: Uint8Array) => opus.length > 0 && (opus[0]! & 0x04) !== 0;

// Frame sizes in µs by configuration (TOC bits 7-3): 0-11 SILK, 12-15
// hybrid, 16-31 CELT.
const SILK_US = [10_000, 20_000, 40_000, 60_000];
const HYBRID_US = [10_000, 20_000];
const CELT_US = [2_500, 5_000, 10_000, 20_000];
const frameUs = (config: number) =>
  config < 12 ? SILK_US[config % 4]! : config < 16 ? HYBRID_US[config % 2]! : CELT_US[config % 4]!;

// Code 0: one frame, 1 and 2: two, 3: the count is in the next byte.
export const opusDurationUs = (opus: Uint8Array) => {
  if (!opus.length) return 0;
  const code = opus[0]! & 3;
  const frames = code === 0 ? 1 : code < 3 ? 2 : (opus[1] ?? 0) & 0x3f;
  return frameUs(opus[0]! >> 3) * frames;
};

// Each second of audio counts as music or speech. It takes a few seconds of
// a high bitrate in a row to switch to music (stereo: at once), and longer back, so the buffer does
// not flap (a quiet passage, a spoken announcement). Silence counts for
// neither: the jukebox stays music between tracks.
const WINDOW_US = 1_000_000;
const MUSIC_BPS = 96_000; // above Mumble's high quality speech (72 kbps)
const TO_MUSIC = 3;
const TO_SPEECH = 10;

// streak: windows in a row that disagree with `music`. The rest counts the
// current window.
export type MusicDetector = { music: boolean; streak: number; bytes: number; us: number; stereo: boolean };

export const createMusicDetector = (): MusicDetector => ({ music: false, streak: 0, bytes: 0, us: 0, stereo: false });

// Runs for each packet, so it changes `d` in place: no allocation.
export const trackMusic = (d: MusicDetector, opus: Uint8Array) => {
  d.bytes += opus.length;
  d.us += opusDurationUs(opus);
  d.stereo ||= isStereo(opus);
  // Stereo is music at once (speech is mono): the buffer is right from the
  // first packet of a stream.
  if (d.stereo && !d.music) {
    d.music = true;
    d.streak = 0;
  }
  if (d.us < WINDOW_US) return;
  const music = d.stereo || (d.bytes * 8e6) / d.us > MUSIC_BPS;
  d.streak = music === d.music ? 0 : d.streak + 1;
  if (d.streak >= (d.music ? TO_SPEECH : TO_MUSIC)) {
    d.music = music;
    d.streak = 0;
  }
  d.bytes = d.us = 0;
  d.stereo = false;
};
