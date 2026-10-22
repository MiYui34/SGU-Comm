// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { mount, unmount } from "svelte";
import ChatView from "../src/sgu/ChatView.svelte";
import { plainText, recentLines, sizeOf, timeOf } from "../src/sgu/chat";
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

  it("carries the quote and the file of a message along", () => {
    const text = textOf({
      1: [
        {
          id: 4,
          channel: 1,
          author: "Ender",
          text: "给了",
          at: 1000,
          reply: { id: 3, author: "Kernel", text: "<b>谁有圆石</b>" },
          upload: { url: "/files/abc/cobble.png", name: "cobble.png", size: 2048, type: "image/png" },
        },
      ],
    });
    const [line] = recentLines(text, 1);
    expect(line?.reply).toEqual({ author: "Kernel", text: "谁有圆石" });
    expect(line?.upload).toEqual({ url: "/files/abc/cobble.png", name: "cobble.png", size: "2 KB" });
  });

  it("says sizes the way people read them", () => {
    expect(sizeOf(999)).toBe("999 B");
    expect(sizeOf(2048)).toBe("2 KB");
    expect(sizeOf(5 * 1024 * 1024)).toBe("5.0 MB");
  });
});

function draw(props: Record<string, unknown>) {
  const target = document.createElement("div");
  document.body.appendChild(target);
  const app = mount(ChatView, { target, props: { text: textOf({}), onSend: () => {}, ...props } });
  return {
    target,
    done: () => {
      unmount(app);
      target.remove();
    },
  };
}

// Svelte 的更新在微任务里：等界面刷完再断言。
const flush = () => new Promise((resolve) => setTimeout(resolve, 10));

describe("the chat tab", () => {
  it("lists the channels and what people said", () => {
    const text = textOf({ 1: [{ id: 1, channel: 1, author: "Kernel", text: "早", at: 1000 }] });
    const { target, done } = draw({ text });
    expect(target.textContent).toContain("综合");
    expect(target.textContent).toContain("建筑");
    expect(target.textContent).toContain("Kernel");
    expect(target.textContent).toContain("早");
    done();
  });

  it("asks for the history of a channel once", async () => {
    const opened: number[] = [];
    const { done } = draw({ onOpen: (id: number) => opened.push(id) });
    await flush();
    expect(opened).toEqual([1]);
    done();
  });

  it("sends what I typed and clears the box", async () => {
    const sent: [number, string][] = [];
    const { target, done } = draw({ text: textOf({ 1: [] }), onSend: (id: number, body: string) => sent.push([id, body]) });
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

  it("answers a line and sends its id along", async () => {
    const sent: unknown[] = [];
    const text = textOf({ 1: [{ id: 7, channel: 1, author: "Kernel", text: "谁在挖矿", at: 1000 }] });
    const { target, done } = draw({
      text,
      onSend: (id: number, body: string, extra?: unknown) => sent.push([id, body, extra]),
    });
    target.querySelector<HTMLButtonElement>("button.reply")?.click();
    await flush();
    expect(target.textContent).toContain("回复 Kernel");

    const box = target.querySelector("textarea");
    if (box === null) throw new Error("no box");
    box.value = "我在";
    box.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();
    target.querySelector<HTMLButtonElement>("button.send")?.click();
    await flush();
    expect(sent).toEqual([[1, "我在", { reply: 7 }]]);
    done();
  });

  it("shows who a message answers and the file it brought", () => {
    const text = textOf({
      1: [
        {
          id: 4,
          channel: 1,
          author: "Ender",
          text: "给了",
          at: 1000,
          reply: { id: 3, author: "Kernel", text: "谁有圆石" },
          upload: { url: "/files/abc/cobble.png", name: "cobble.png", size: 2048, type: "image/png" },
        },
      ],
    });
    const { target, done } = draw({ text });
    expect(target.textContent).toContain("谁有圆石");
    const link = target.querySelector<HTMLAnchorElement>("a.file");
    expect(link?.getAttribute("href")).toBe("/files/abc/cobble.png");
    expect(link?.textContent).toContain("cobble.png");
    expect(link?.textContent).toContain("2 KB");
    done();
  });

  it("puts a file on the message it is sent with", async () => {
    const sent: unknown[] = [];
    const { target, done } = draw({
      text: textOf({ 1: [] }),
      onSend: (id: number, body: string, extra?: unknown) => sent.push([id, body, extra]),
      onUpload: async () => "key-1",
    });
    const picker = target.querySelector<HTMLInputElement>("input[type=file]");
    if (picker === null) throw new Error("no picker");
    const file = new File(["hello"], "note.txt", { type: "text/plain" });
    Object.defineProperty(picker, "files", { value: [file], configurable: true });
    picker.dispatchEvent(new Event("change", { bubbles: true }));
    await flush();
    expect(target.textContent).toContain("note.txt");

    target.querySelector<HTMLButtonElement>("button.send")?.click();
    await flush();
    expect(sent).toEqual([[1, "", { upload: "key-1" }]]);
    done();
  });

  it("refuses a file above the server's limit", async () => {
    const uploaded: File[] = [];
    const text = { ...textOf({ 1: [] }), uploadMax: 4 } as TextState;
    const { target, done } = draw({
      text,
      onUpload: async (file: File) => {
        uploaded.push(file);
        return "key-1";
      },
    });
    const picker = target.querySelector<HTMLInputElement>("input[type=file]");
    if (picker === null) throw new Error("no picker");
    Object.defineProperty(picker, "files", {
      value: [new File(["too long"], "big.txt", { type: "text/plain" })],
      configurable: true,
    });
    picker.dispatchEvent(new Event("change", { bubbles: true }));
    await flush();
    expect(uploaded).toEqual([]);
    expect(target.textContent).toContain("文件不能超过 4 B");
    done();
  });

  it("does not offer a box where I cannot speak", () => {
    const { target, done } = draw({ text: textOf({ 1: [] }), canWrite: () => false });
    expect(target.querySelector("textarea")).toBeNull();
    expect(target.textContent).toContain("这个频道你不能发言");
    done();
  });
});
