// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { mount, unmount } from "svelte";
import ChatView from "../src/sgu/ChatView.svelte";
import { plainText, recentLines, timeOf } from "../src/sgu/chat";
import type { TextState } from "../src/lib/textChannels";

const textOf = (messages: Record<number, unknown[]>, loaded: Record<number, boolean> = {}) =>
  ({
    available: true,
    channels: [
      { id: 1, name: "综合" },
      { id: 2, name: "建筑" },
    ],
    messages,
    loaded,
  }) as unknown as TextState;

describe("chat text", () => {
  it("shows a message as plain words, not as markup", () => {
    expect(plainText("<p>你好<br>世界 &amp; 大家</p>")).toBe("你好\n世界 & 大家");
    expect(plainText("<b>粗</b><script>坏</script>")).toBe("粗坏");
  });

  it("keeps the last lines in the order they were said", () => {
    const text = textOf({
      1: [
        { id: 1, channel: 1, author: "Kernel", text: "<p>早</p>", at: 1000 },
        { id: 2, channel: 1, author: "Ender", text: "来了", at: 2000 },
        { id: 3, channel: 1, author: "Kernel", text: "开工", at: 3000 },
      ],
    });
    expect(recentLines(text, 1, 2).map((line) => [line.author, line.text])).toEqual([
      ["Ender", "来了"],
      ["Kernel", "开工"],
    ]);
  });

  it("has nothing to show in a channel that was never used", () => {
    expect(recentLines(textOf({}), 9)).toEqual([]);
  });

  it("prints the time of day only", () => {
    expect(timeOf(new Date(2026, 9, 22, 9, 5).getTime())).toMatch(/^\d{2}:\d{2}$/);
  });
});

function draw(text: TextState, onSend: (id: number, body: string) => void, onOpen?: (id: number) => void) {
  const target = document.createElement("div");
  document.body.appendChild(target);
  const app = mount(ChatView, { target, props: { text, onSend, ...(onOpen && { onOpen }) } });
  return {
    target,
    done: () => {
      unmount(app);
      target.remove();
    },
  };
}

describe("the chat tab", () => {
  // Svelte 的更新在微任务里：等界面刷完再断言。
  const flush = () => new Promise((resolve) => setTimeout(resolve, 10));

  it("lists the channels and what people said", () => {
    const text = textOf({ 1: [{ id: 1, channel: 1, author: "Kernel", text: "早", at: 1000 }] });
    const { target, done } = draw(text, () => {});
    expect(target.textContent).toContain("综合");
    expect(target.textContent).toContain("建筑");
    expect(target.textContent).toContain("Kernel");
    expect(target.textContent).toContain("早");
    done();
  });

  it("asks for the history of a channel once", async () => {
    const opened: number[] = [];
    const { done } = draw(textOf({}), () => {}, (id) => opened.push(id));
    await flush();
    expect(opened).toEqual([1]);
    done();
  });

  it("sends what I typed and clears the box", async () => {
    const sent: [number, string][] = [];
    const { target, done } = draw(textOf({ 1: [] }), (id, body) => sent.push([id, body]));
    const box = target.querySelector("textarea");
    if (box === null) throw new Error("no box");
    box.value = "  今晚八点集合  ";
    box.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();
    target.querySelector<HTMLButtonElement>("button.send")?.click();
    await flush();
    expect(sent).toEqual([[1, "今晚八点集合"]]);
    expect(box.value).toBe("");
    done();
  });
});
