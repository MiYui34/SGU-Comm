import { describe, expect, it } from "vitest";
import { afterUpload, canAttach } from "../src/lib/composer";

describe("canAttach", () => {
  it("takes a file only where uploads work and none is running", () => {
    expect(canAttach(true, false)).toBe(true);
    expect(canAttach(true, true)).toBe(false);
    expect(canAttach(false, false)).toBe(false);
  });
});

describe("afterUpload", () => {
  const file = new File(["x"], "a.png");
  const sent = { target: "#general", caption: "look", file };
  const now = { target: "#general", draft: "look ", file };

  it("clears the file and caption after a success", () => {
    expect(afterUpload(true, sent, now)).toEqual({ file: true, caption: true });
  });

  it("keeps both after a failure, for a retry", () => {
    expect(afterUpload(false, sent, now)).toEqual({ file: false, caption: false });
  });

  it("keeps a caption that was changed during the upload", () => {
    expect(afterUpload(true, sent, { ...now, draft: "look at this" }).caption).toBe(false);
  });

  it("keeps the text of another chat", () => {
    expect(afterUpload(true, sent, { target: "#random", draft: "look", file: undefined })).toEqual({ file: false, caption: false });
  });
});
