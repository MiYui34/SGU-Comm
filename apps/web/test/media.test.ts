import { describe, expect, it } from "vitest";
import { formatDuration, mediaKind } from "../src/lib/media";

describe("mediaKind", () => {
  it("plays audio and video by type, and older uploads by name", () => {
    expect(mediaKind({ type: "audio/mpeg", name: "x" })).toBe("audio");
    expect(mediaKind({ type: "video/mp4", name: "x" })).toBe("video");
    expect(mediaKind({ type: "application/octet-stream", name: "02 Song.MP3" })).toBe("audio");
    expect(mediaKind({ type: "application/octet-stream", name: "clip.webm" })).toBe("video");
    expect(mediaKind({ type: "application/octet-stream", name: "notes.txt" })).toBeNull();
    expect(mediaKind({ type: "image/png", name: "a.png" })).toBeNull();
  });
});

describe("formatDuration", () => {
  it("shows minutes and seconds (hours when long)", () => {
    expect(formatDuration(0)).toBe("0:00");
    expect(formatDuration(211.6)).toBe("3:31");
    expect(formatDuration(3725)).toBe("1:02:05");
    expect(formatDuration(NaN)).toBe("0:00");
  });
});
