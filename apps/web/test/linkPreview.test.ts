import { describe, expect, it } from "vitest";
import { firstLink, previewsToAsk } from "../src/lib/linkPreview";
import { initialTextState, parseServerEvent, reduceText, type TextMessage } from "../src/lib/textChannels";

const msg = (id: number, text: string): TextMessage => ({ id, channel: 1, author: "a", text, at: 1 });

describe("link previews", () => {
  it("finds the first link as the server does", () => {
    expect(firstLink("see https://x.org/a?b=1.")).toBe("https://x.org/a?b=1");
    expect(firstLink("<https://x.org/a> and http://y")).toBe("https://x.org/a");
    expect(firstLink("**https://x.org**")).toBe("https://x.org");
    expect(firstLink("javascript:alert(1)")).toBeNull();
  });

  it("asks once for messages with a link and no answer yet", () => {
    const msgs = [msg(1, "https://a.org"), msg(2, "plain"), msg(3, "https://b.org"), msg(4, "https://c.org")];
    expect(previewsToAsk(msgs, { 3: null }, new Set([4]))).toEqual([1]);
  });

  it("keeps the server's answers", () => {
    const ev = parseServerEvent(
      JSON.stringify({ type: "preview", channel: 1, message: 5, preview: { url: "https://a.org", title: "A", image: "/api/link-image/k" } }),
    );
    expect(ev).toEqual({ type: "preview", message: 5, preview: { url: "https://a.org", title: "A", image: "/api/link-image/k" } });
    expect(parseServerEvent('{"type":"preview","message":6,"preview":null}')).toEqual({ type: "preview", message: 6, preview: null });
    expect(parseServerEvent('{"type":"preview","message":6,"preview":{"url":"javascript:x","title":"A"}}')).toBeNull();
    const state = reduceText(initialTextState, ev!);
    expect(state.previews[5]?.title).toBe("A");
  });
});
