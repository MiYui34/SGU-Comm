import { describe, expect, it } from "vitest";
import { asSpoiler, formatDuration, mediaKind, spoilerUpload } from "../src/lib/media";

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

describe("spoiler attachments", () => {
  it("are marked by name, as in Discord", () => {
    const file = new File(["x"], "cat.png", { type: "image/png" });
    expect(asSpoiler(file, false)).toBe(file);
    const spoiler = asSpoiler(file, true);
    expect([spoiler.name, spoiler.type]).toEqual(["SPOILER_cat.png", "image/png"]);
    expect(asSpoiler(spoiler, true).name).toBe("SPOILER_cat.png");
  });

  it("show without the prefix", () => {
    const u = { url: "/f/1", name: "SPOILER_cat.png", type: "image/png", size: 1 };
    expect(spoilerUpload(u)).toEqual({ ...u, name: "cat.png", spoiler: true });
    expect(spoilerUpload({ ...u, name: "cat.png" })).toEqual({ ...u, name: "cat.png" });
  });
});
