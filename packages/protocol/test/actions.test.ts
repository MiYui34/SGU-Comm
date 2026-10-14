import { describe, expect, it } from "vitest";
import { decodeFrames } from "../src/framing";
import { decodeMessage } from "../src/messages";
import { escapeHtml, joinChannel, sendText, setSelfState } from "../src/actions";

const decodeOne = (bytes: Uint8Array) => {
  const { frames } = decodeFrames(bytes);
  expect(frames).toHaveLength(1);
  return decodeMessage(frames[0]!);
};

describe("escapeHtml", () => {
  it("escapes characters that have meaning in HTML", () => {
    expect(escapeHtml(`<b>"Tom" & 'Jerry'</b>`)).toBe(
      "&lt;b&gt;&quot;Tom&quot; &amp; &#39;Jerry&#39;&lt;/b&gt;",
    );
  });
});

describe("sendText", () => {
  it("sends escaped text to one channel", () => {
    const msg = decodeOne(sendText(4, "a < b"));
    if (msg.type !== "TextMessage") throw new Error();
    expect(msg.message.channelId).toEqual([4]);
    expect(msg.message.message).toBe("a &lt; b");
  });
});

describe("joinChannel", () => {
  it("moves the own session to a channel", () => {
    const msg = decodeOne(joinChannel(9, 0));
    if (msg.type !== "UserState") throw new Error();
    expect(msg.message).toMatchObject({ session: 9, channelId: 0 });
  });
});

describe("setSelfState", () => {
  it("sends self mute and self deaf", () => {
    const msg = decodeOne(setSelfState(9, { selfMute: true, selfDeaf: false }));
    if (msg.type !== "UserState") throw new Error();
    expect(msg.message).toMatchObject({ session: 9, selfMute: true, selfDeaf: false });
  });
});
