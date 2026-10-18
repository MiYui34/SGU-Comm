import { describe, expect, it } from "vitest";
import { afterUpload, canAttach, switchDraft } from "../src/lib/composer";

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
  const now = { target: "#general", draft: "look ", file, drafts: new Map<string, string>() };

  it("clears the file and caption after a success", () => {
    expect(afterUpload(true, sent, now)).toEqual({ file: true, caption: true, kept: false });
  });

  it("keeps both after a failure, for a retry", () => {
    expect(afterUpload(false, sent, now)).toEqual({ file: false, caption: false, kept: false });
  });

  it("keeps a caption that was changed during the upload", () => {
    expect(afterUpload(true, sent, { ...now, draft: "look at this" }).caption).toBe(false);
  });

  it("keeps the text of another chat", () => {
    const other = { ...now, target: "#random", draft: "look", file: undefined };
    expect(afterUpload(true, sent, other)).toEqual({ file: false, caption: false, kept: false });
  });

  it("forgets the sent caption kept for the chat it was sent in", () => {
    const other = { ...now, target: "#random", draft: "", file: undefined };
    expect(afterUpload(true, sent, { ...other, drafts: new Map([["#general", "look"]]) }).kept).toBe(true);
    expect(afterUpload(false, sent, { ...other, drafts: new Map([["#general", "look"]]) }).kept).toBe(false);
    expect(afterUpload(true, sent, { ...other, drafts: new Map([["#general", "look again"]]) }).kept).toBe(false);
  });
});

describe("switchDraft", () => {
  it("keeps the draft of the chat we leave and brings back the other one", () => {
    const left = switchDraft(new Map(), "#general", "#random", "half a thought");
    expect(left).toEqual({ drafts: new Map([["#general", "half a thought"]]), draft: "" });
    const back = switchDraft(left.drafts, "#random", "#general", "");
    expect(back).toEqual({ drafts: new Map(), draft: "half a thought" });
  });

  it("keeps no empty drafts", () => {
    expect(switchDraft(new Map([["#general", "old"]]), "#general", "#random", "  ").drafts).toEqual(new Map());
  });

  it("does not change the map it got", () => {
    const drafts = new Map([["#random", "hi"]]);
    switchDraft(drafts, "#general", "#random", "x");
    expect(drafts).toEqual(new Map([["#random", "hi"]]));
  });
});
