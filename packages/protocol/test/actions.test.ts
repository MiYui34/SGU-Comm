import { describe, expect, it } from "vitest";
import { decodeFrames } from "../src/framing";
import { decodeMessage } from "../src/messages";
import { escapeHtml, joinChannel, requestDescriptions, sendText, setSelfState } from "../src/actions";

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
    const msg = decodeOne(sendText({ channelId: 4 }, "a < b"));
    if (msg.type !== "TextMessage") throw new Error();
    expect(msg.message.channelId).toEqual([4]);
    expect(msg.message.session).toEqual([]);
    expect(msg.message.message).toBe("a &lt; b");
  });

  it("sends a private message to one user", () => {
    const msg = decodeOne(sendText({ session: 9 }, "psst"));
    if (msg.type !== "TextMessage") throw new Error();
    expect(msg.message.session).toEqual([9]);
    expect(msg.message.channelId).toEqual([]);
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

describe("requestDescriptions", () => {
  it("asks for channel description blobs", () => {
    const msg = decodeOne(requestDescriptions([0, 4]));
    if (msg.type !== "RequestBlob") throw new Error();
    expect(msg.message.channelDescription).toEqual([0, 4]);
  });
});
