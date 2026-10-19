import { describe, expect, it } from "vitest";
import { createMusicDetector, isStereo, opusDurationUs, trackMusic, type MusicDetector } from "../src/lib/voice/music";

// An Opus packet: the TOC byte, then filler up to `bytes`.
const packet = (toc: number, bytes = 1, second = 0) => {
  const p = new Uint8Array(bytes);
  p[0] = toc;
  if (bytes > 1) p[1] = second;
  return p;
};
// 20 ms CELT fullband frames, mono and stereo.
const MONO = 0xf8;
const STEREO = 0xfc;
// Bytes per 20 ms: 40 kbps (our speech) and 128 kbps (music mode).
const VOICE_BYTES = 100;
const MUSIC_BYTES = 320;

// Feeds `seconds` of 20 ms packets.
const feed = (d: MusicDetector, toc: number, bytes: number, seconds: number) => {
  for (let i = 0; i < seconds * 50; i++) trackMusic(d, packet(toc, bytes));
  return d.music;
};

describe("Mumble's high quality speech", () => {
  it("stays speech at 72 kbps mono", () => {
    expect(feed(createMusicDetector(), MONO, 180, 5)).toBe(false);
  });
});

describe("isStereo", () => {
  it("reads the stereo flag of the TOC byte", () => {
    expect(isStereo(packet(STEREO))).toBe(true);
    expect(isStereo(packet(MONO))).toBe(false);
    expect(isStereo(new Uint8Array(0))).toBe(false);
  });
});

describe("opusDurationUs", () => {
  it("reads the frame size from the configuration", () => {
    expect(opusDurationUs(packet(MONO))).toBe(20_000);
    expect(opusDurationUs(packet(1 << 3))).toBe(20_000); // SILK
    expect(opusDurationUs(packet(3 << 3))).toBe(60_000);
    expect(opusDurationUs(packet(12 << 3))).toBe(10_000); // hybrid
    expect(opusDurationUs(packet(16 << 3))).toBe(2_500); // CELT
  });

  it("counts the frames in the packet", () => {
    expect(opusDurationUs(packet(MONO | 1))).toBe(40_000);
    expect(opusDurationUs(packet(MONO | 3, 2, 3))).toBe(60_000);
    expect(opusDurationUs(packet(MONO | 3))).toBe(0);
    expect(opusDurationUs(new Uint8Array(0))).toBe(0);
  });
});

describe("trackMusic", () => {
  it("keeps speech as speech", () => {
    expect(feed(createMusicDetector(), MONO, VOICE_BYTES, 20)).toBe(false);
  });

  it("takes stereo for music after 3 s", () => {
    const d = createMusicDetector();
    expect(feed(d, STEREO, VOICE_BYTES, 2)).toBe(false);
    expect(feed(d, STEREO, VOICE_BYTES, 1)).toBe(true);
  });

  it("takes a high bitrate for music", () => {
    expect(feed(createMusicDetector(), MONO, MUSIC_BYTES, 3)).toBe(true);
  });

  it("goes back to speech only after 10 s of it", () => {
    const d = createMusicDetector();
    feed(d, STEREO, MUSIC_BYTES, 3);
    expect(feed(d, MONO, VOICE_BYTES, 9)).toBe(true);
    expect(feed(d, MONO, VOICE_BYTES, 1)).toBe(false);
  });

  it("needs the seconds in a row, so it does not flap", () => {
    const d = createMusicDetector();
    feed(d, STEREO, VOICE_BYTES, 2);
    feed(d, MONO, VOICE_BYTES, 1);
    expect(feed(d, STEREO, VOICE_BYTES, 2)).toBe(false);
  });
});
