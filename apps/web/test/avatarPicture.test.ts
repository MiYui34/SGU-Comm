import { describe, expect, it } from "vitest";
import { emptyUser, initialState, type ServerState } from "@kallisti/protocol";
import { imageType, pictureUrl, squareCrop, texturesToFetch } from "../src/lib/avatar";

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
    const png = bytes(0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a);
    const url = pictureUrl(png);
    expect(url).toBe("data:image/png;base64,iVBORw0KGgo=");
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
