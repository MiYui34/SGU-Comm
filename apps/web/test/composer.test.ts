import { describe, expect, it } from "vitest";
import { afterUpload, canAttach, chatKeyOf, switchDraft, tooLarge } from "../src/lib/composer";

describe("canAttach", () => {
  it("takes a file only where uploads work and none is running", () => {
    expect(canAttach(true, false)).toBe(true);
    expect(canAttach(true, true)).toBe(false);
    expect(canAttach(false, false)).toBe(false);
  });
});

describe("afterUpload", () => {
  const file = new File(["x"], "a.png");
  const sent = { chat: "text 1", caption: "look", file };
  const none = new Map<string, string>();
  const now = { chat: "text 1", draft: "look ", file, drafts: none };

  it("clears the file and caption after a success", () => {
    expect(afterUpload(true, sent, now)).toEqual({ file: true, caption: true, drafts: none });
  });

  it("keeps both after a failure, for a retry", () => {
    expect(afterUpload(false, sent, now)).toEqual({ file: false, caption: false, drafts: none });
  });

  it("keeps a caption that was changed during the upload", () => {
    expect(afterUpload(true, sent, { ...now, draft: "look at this" }).caption).toBe(false);
  });

  it("keeps the text of another chat", () => {
    const other = { ...now, chat: "text 2", draft: "look", file: undefined };
    expect(afterUpload(true, sent, other)).toEqual({ file: false, caption: false, drafts: none });
  });

  it("forgets the sent caption kept for the chat it was sent in", () => {
    const other = { ...now, chat: "text 2", draft: "", file: undefined };
    const kept = new Map([["text 1", "look"], ["text 3", "hi"]]);
    expect(afterUpload(true, sent, { ...other, drafts: kept }).drafts).toEqual(new Map([["text 3", "hi"]]));
    expect(afterUpload(false, sent, { ...other, drafts: kept }).drafts).toBe(kept);
    const changed = new Map([["text 1", "look again"]]);
    expect(afterUpload(true, sent, { ...other, drafts: changed }).drafts).toBe(changed);
  });
});

describe("chatKeyOf", () => {
  it("tells chats apart by id, private chats by name", () => {
    expect(chatKeyOf({ kind: "text", id: 3 }, 0)).toBe("text 3");
    expect(chatKeyOf({ kind: "voice" }, 5)).not.toBe(chatKeyOf({ kind: "voice" }, 6));
    expect(chatKeyOf({ kind: "dm", session: 2, name: "bob" }, 5)).toBe(chatKeyOf({ kind: "dm", session: null, name: "bob" }, 0));
    expect(chatKeyOf({ kind: "dm", session: 2, name: "5" }, 5)).not.toBe(chatKeyOf({ kind: "voice" }, 5));
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

describe("tooLarge", () => {
  it("names the limit for a file over it", () => {
    expect(tooLarge(10 * 1024 * 1024 + 1, 10 * 1024 * 1024)).toBe("The file is too large (at most 10 MB).");
    expect(tooLarge(10 * 1024 * 1024, 10 * 1024 * 1024)).toBeNull();
  });
  it("lets the server decide without a known limit", () => {
    expect(tooLarge(1e12, undefined)).toBeNull();
  });
});
