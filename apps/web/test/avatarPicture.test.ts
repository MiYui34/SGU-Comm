import { describe, expect, it } from "vitest";
import { emptyUser, initialState, type ServerState } from "@kallisti/protocol";
import { imageSize, imageType, pictureLookup, pictureUrl, squareCrop, texturesToFetch } from "../src/lib/avatar";

const bytes = (...b: number[]) => new Uint8Array(b);

describe("avatar pictures", () => {
  it("knows image types by their first bytes", () => {
    expect(imageType(bytes(0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a))).toBe("image/png");
    expect(imageType(bytes(0xff, 0xd8, 0xff, 0xe0))).toBe("image/jpeg");
    expect(imageType(new TextEncoder().encode("GIF89a"))).toBe("image/gif");
    expect(imageType(new TextEncoder().encode("RIFF\0\0\0\0WEBPVP8 "))).toBe("image/webp");
    // Old Mumble raw textures, SVG (scripts) and junk are not shown.
    expect(imageType(new TextEncoder().encode("<svg onload=x>"))).toBeNull();
    expect(imageType(bytes(1, 2, 3))).toBeNull();
  });

  it("makes a data URL once per picture", () => {
    const png = bytes(0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 13, 0x49, 0x48, 0x44, 0x52, 0, 0, 0, 1, 0, 0, 0, 1);
    const url = pictureUrl(png);
    expect(url).toBe("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB");
    expect(pictureUrl(png)).toBe(url);
    expect(pictureUrl(bytes(1, 2))).toBeNull();
    expect(pictureUrl(null)).toBeNull();
  });

  it("crops the middle square", () => {
    expect(squareCrop(400, 200)).toEqual({ x: 100, y: 0, size: 200 });
    expect(squareCrop(200, 300)).toEqual({ x: 0, y: 50, size: 200 });
  });
});

describe("texturesToFetch", () => {
  const user = (session: number, textureHash: string | null) => ({ ...emptyUser(session), textureHash });
  const state = (...users: ReturnType<typeof user>[]): ServerState => ({
    ...initialState,
    status: "synced",
    users: Object.fromEntries(users.map((u) => [u.session, u])),
  });

  it("asks once per user and picture, and forgets users who left", () => {
    const first = texturesToFetch(state(user(1, "aa"), user(2, null)), new Set());
    expect(first.sessions).toEqual([1]);
    expect(texturesToFetch(state(user(1, "aa")), first.asked).sessions).toEqual([]);
    expect(texturesToFetch(state(user(1, "bb")), first.asked).sessions).toEqual([1]);
    // Session 1 left; a new user with that number (Murmur reuses them).
    const gone = texturesToFetch(state(), first.asked);
    expect(gone.asked.size).toBe(0);
    expect(texturesToFetch(state(user(1, "aa")), gone.asked).sessions).toEqual([1]);
  });
});

describe("imageSize", () => {
  const png = (w: number, h: number) => {
    const b = new Uint8Array(24);
    b.set([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 13, 0x49, 0x48, 0x44, 0x52]);
    new DataView(b.buffer).setUint32(16, w);
    new DataView(b.buffer).setUint32(20, h);
    return b;
  };
  const gif = (w: number, h: number) => {
    const b = new Uint8Array(10);
    b.set(new TextEncoder().encode("GIF89a"));
    new DataView(b.buffer).setUint16(6, w, true);
    new DataView(b.buffer).setUint16(8, h, true);
    return b;
  };
  // SOI, an APP0 segment of 4 bytes, then SOF0 with height and width.
  const jpeg = (w: number, h: number) =>
    new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0, 4, 0, 0, 0xff, 0xc0, 0, 11, 8, h >> 8, h & 255, w >> 8, w & 255, 3]);
  const webpLossless = (w: number, h: number) => {
    const b = new Uint8Array(30);
    b.set(new TextEncoder().encode("RIFF\0\0\0\0WEBPVP8L"));
    b[20] = 0x2f;
    const bits = (w - 1) | ((h - 1) << 14);
    new DataView(b.buffer).setUint32(21, bits, true);
    return b;
  };

  it("reads the size from the header", () => {
    expect(imageSize(png(128, 64))).toEqual({ width: 128, height: 64 });
    expect(imageSize(gif(300, 200))).toEqual({ width: 300, height: 200 });
    expect(imageSize(jpeg(640, 480))).toEqual({ width: 640, height: 480 });
    expect(imageSize(webpLossless(100, 50))).toEqual({ width: 100, height: 50 });
    expect(imageSize(new Uint8Array([1, 2, 3]))).toBeNull();
  });

  it("shows no picture for very large ones", () => {
    expect(pictureUrl(png(128, 128))).not.toBeNull();
    expect(pictureUrl(png(6000, 5000))).toBeNull();
  });

  it("finds online users' pictures, else the saved one of registered users", () => {
    const lookup = pictureLookup(new Map([["alice", "data:alice"], ["bob", null]]), "https://chat.example");
    expect(lookup("alice", true)).toBe("data:alice");
    // Online without a picture: none (no request).
    expect(lookup("bob", true)).toBeNull();
    expect(lookup("carol / x", true)).toBe("https://chat.example/api/avatars/carol%20%2F%20x");
    // Guests who left have none.
    expect(lookup("dave", false)).toBeNull();
  });
});
