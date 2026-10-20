import { describe, expect, it } from "vitest";
import { asSpoiler, chatSrc, downloadUrl, formatDuration, imageBox, mediaKind, mediaRatio, shownLength, spoilerUpload, videoPreload } from "../src/lib/media";

describe("previews", () => {
  it("show a picture's preview in the chat, else the picture", () => {
    expect(chatSrc({ url: "/files/k/a.png", preview: "/files/k/a.png?preview" })).toBe("/files/k/a.png?preview");
    expect(chatSrc({ url: "/files/k/a.png" })).toBe("/files/k/a.png");
  });

  it("give a video a poster and load it only when played", () => {
    expect(videoPreload("/files/k/a.mp4?preview")).toBe("none");
    // Without one: its metadata and first frame.
    expect(videoPreload(undefined)).toBe("metadata");
  });
});

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

describe("shownLength", () => {
  it("is the player's length, else the server's", () => {
    expect(shownLength(12.5, 3000)).toBe(12.5);
    expect(shownLength(0, 3000)).toBe(3);
    expect(shownLength(Infinity, 3000)).toBe(3); // a recording without a length
    expect(shownLength(NaN)).toBe(0);
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

describe("media boxes", () => {
  it("keep a picture's size, scaled down to the largest box", () => {
    expect(imageBox({ width: 40, height: 30 })).toEqual({ width: 40, height: 30 });
    expect(imageBox({ width: 1600, height: 900 })).toEqual({ width: 400, height: 225 });
    expect(imageBox({ width: 900, height: 1600 })).toEqual({ width: 169, height: 300 });
    expect(imageBox({ width: 10000, height: 1 })).toEqual({ width: 400, height: 1 });
    expect(imageBox({ width: 640, height: 480 }, { width: 160, height: 300 })).toEqual({ width: 160, height: 120 });
  });

  it("are unknown without a size", () => {
    expect(imageBox({})).toBeNull();
    expect(imageBox({ width: 0, height: 10 })).toBeNull();
    expect(mediaRatio({ width: 10 })).toBeUndefined();
  });

  it("give videos their aspect ratio", () => {
    expect(mediaRatio({ width: 1280, height: 720 })).toBe("1280 / 720");
  });
});

describe("downloadUrl", () => {
  it("asks the server for an attachment", () => {
    expect(downloadUrl("https://x.example/files/abc/clip.mp4")).toBe("https://x.example/files/abc/clip.mp4?download");
    expect(downloadUrl("/files/abc/a.mp4?x=1")).toBe("/files/abc/a.mp4?x=1&download");
  });
});
