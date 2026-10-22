// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { mount, unmount } from "svelte";
import ChatView from "../src/sgu/ChatView.svelte";
import { plainText, recentLines, sizeOf, timeOf } from "../src/sgu/chat";
import type { TextState } from "../src/lib/textChannels";

// jsdom 的 <dialog> 还没有模态方法，大图一开就调它。
HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) {
  this.open = true;
};
HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) {
  this.open = false;
  this.dispatchEvent(new Event("close"));
};

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
    expect(line?.upload).toEqual({ url: "/files/abc/cobble.png", name: "cobble.png", type: "image/png", size: "2 KB" });
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
    // 图片就当图片显示，不再是一个下载链接。
    const img = target.querySelector<HTMLImageElement>(".shot img");
    expect(img?.getAttribute("src")).toBe("/files/abc/cobble.png");
    expect(img?.getAttribute("alt")).toBe("cobble.png");
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

const said = (id: number, text: string, extra: Record<string, unknown> = {}) => ({
  id,
  channel: 1,
  author: "Kernel",
  text,
  at: 1000,
  ...extra,
});

describe("reactions, edits and pins", () => {
  it("counts a reaction and knows whether it is mine", () => {
    const text = textOf({
      1: [said(1, "早", { reactions: [{ emoji: "👍", users: ["Kernel", "Ender"], ids: [7, 8] }] })],
    });
    expect(recentLines(text, 1, 60, 8)[0]?.reactions).toEqual([{ emoji: "👍", count: 2, mine: true }]);
    expect(recentLines(text, 1, 60, 9)[0]?.reactions?.[0]?.mine).toBe(false);
  });

  it("marks a message that was edited or pinned", () => {
    const text = textOf({ 1: [said(1, "改过", { edited: 2000, pinned: 3000 })] });
    const [line] = recentLines(text, 1);
    expect(line?.edited).toBe(true);
    expect(line?.pinned).toBe(true);
  });

  it("shows the marks on the line itself", () => {
    const text = textOf({ 1: [said(1, "别忘带火把", { pinned: 3000, edited: 4000 })] });
    const { target, done } = draw({ text });
    expect(target.querySelector(".line.pinned")).not.toBeNull();
    expect(target.textContent).toContain("置顶");
    expect(target.textContent).toContain("已编辑");
    done();
  });

  it("adds a reaction from the palette", async () => {
    const calls: unknown[] = [];
    const { target, done } = draw({
      text: textOf({ 1: [said(1, "早")] }),
      onReact: (id: number, emoji: string, on: boolean) => calls.push([id, emoji, on]),
    });
    target.querySelector<HTMLButtonElement>("button.more")?.click();
    await flush();
    const palette = target.querySelector(".palette");
    expect(palette).not.toBeNull();
    palette?.querySelector("button")?.click();
    await flush();
    expect(calls).toEqual([[1, "👍", true]]);
    expect(target.querySelector(".palette")).toBeNull();
    done();
  });

  it("takes back a reaction I already gave", async () => {
    const calls: unknown[] = [];
    const text = textOf({
      1: [said(1, "早", { reactions: [{ emoji: "👍", users: ["Ender"], ids: [8] }] })],
    });
    const { target, done } = draw({
      text,
      myId: 8,
      onReact: (id: number, emoji: string, on: boolean) => calls.push([id, emoji, on]),
    });
    const chip = target.querySelector<HTMLButtonElement>("button.react");
    expect(chip?.classList.contains("mine")).toBe(true);
    chip?.click();
    await flush();
    expect(calls).toEqual([[1, "👍", false]]);
    done();
  });

  it("edits a line I wrote", async () => {
    const edited: unknown[] = [];
    const { target, done } = draw({
      text: textOf({ 1: [said(1, "旧话")] }),
      myName: "Kernel",
      onEdit: (id: number, body: string) => edited.push([id, body]),
    });
    target.querySelector<HTMLButtonElement>("button.edit-btn")?.click();
    await flush();
    const box = target.querySelector<HTMLTextAreaElement>(".edit textarea");
    if (box === null) throw new Error("no edit box");
    box.value = "  新话  ";
    box.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();
    target.querySelector<HTMLButtonElement>(".edit-acts .save")?.click();
    await flush();
    expect(edited).toEqual([[1, "新话"]]);
    expect(target.querySelector(".edit")).toBeNull();
    done();
  });

  it("hides editing on somebody else's line", () => {
    const { target, done } = draw({
      text: textOf({ 1: [said(1, "别人的话")] }),
      myName: "Steve_SGU",
      onEdit: () => {},
    });
    expect(target.querySelector("button.edit-btn")).toBeNull();
    done();
  });

  it("pins and unpins a line", async () => {
    const calls: unknown[] = [];
    const loose = draw({
      text: textOf({ 1: [said(1, "要记的")] }),
      onPin: (id: number, on: boolean) => calls.push([id, on]),
    });
    loose.target.querySelector<HTMLButtonElement>("button.pin")?.click();
    await flush();
    loose.done();

    const pinned = draw({
      text: textOf({ 1: [said(1, "要记的", { pinned: 3000 })] }),
      onPin: (id: number, on: boolean) => calls.push([id, on]),
    });
    const button = pinned.target.querySelector<HTMLButtonElement>("button.pin");
    expect(button?.textContent?.trim()).toBe("取消置顶");
    button?.click();
    await flush();
    pinned.done();
    expect(calls).toEqual([
      [1, true],
      [1, false],
    ]);
  });
});

describe("searching, pins and threads", () => {
  const withOne = () =>
    textOf({
      1: [{ id: 1, channel: 1, author: "Kernel", text: "早", at: 1000 }],
      2: [{ id: 2, channel: 2, author: "Ender", text: "在", at: 2000 }],
    });

  it("asks the server to search when the box is entered", async () => {
    const seen: string[] = [];
    const { target, done } = draw({ text: withOne(), onSearch: (query: string) => seen.push(query) });
    const box = target.querySelector<HTMLInputElement>(".find");
    if (box === null) throw new Error("no search box");
    box.value = "from:Kernel 圆石";
    box.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    await flush();
    expect(seen).toEqual(["from:Kernel 圆石"]);
    done();
  });

  it("opens pins and threads for the channel it is looking at", async () => {
    const pins: number[] = [];
    const threads: number[] = [];
    const { target, done } = draw({
      text: withOne(),
      onPins: (id: number) => pins.push(id),
      onThreads: (id: number) => threads.push(id),
    });
    const buttons = [...target.querySelectorAll<HTMLButtonElement>(".tools button")];
    buttons[0]?.click();
    buttons[1]?.click();
    await flush();
    expect(pins).toEqual([1]);
    expect(threads).toEqual([1]);
    done();
  });

  it("only offers to delete what I said", async () => {
    const removed: number[] = [];
    const { target, done } = draw({
      text: withOne(),
      myName: "Kernel",
      onDelete: (id: number) => removed.push(id),
    });
    const dels = [...target.querySelectorAll<HTMLButtonElement>(".del")];
    expect(dels.length).toBe(1);
    dels[0]?.click();
    await flush();
    expect(removed).toEqual([1]);
    done();
  });

  it("starts a thread from a message", async () => {
    const started: number[] = [];
    const { target, done } = draw({ text: withOne(), onThread: (id: number) => started.push(id) });
    const first = target.querySelector<HTMLButtonElement>("button.thread");
    if (first === null) throw new Error("no thread button");
    first.click();
    await flush();
    expect(started).toEqual([1]);
    done();
  });
});
describe("pictures, sound and video", () => {
  const withFile = (upload: Record<string, unknown>) =>
    textOf({ 1: [{ id: 9, channel: 1, author: "Ender", text: "", at: 1000, upload }] });

  it("shows a picture and opens it bigger", async () => {
    const { target, done } = draw({
      text: withFile({
        url: "/files/a/sunset.png",
        name: "sunset.png",
        type: "image/png",
        size: 4096,
        width: 800,
        height: 600,
      }),
    });
    const shot = target.querySelector<HTMLButtonElement>("button.shot");
    expect(shot).not.toBeNull();
    const img = target.querySelector<HTMLImageElement>(".shot img");
    expect(img?.getAttribute("src")).toBe("/files/a/sunset.png");
    // 800x600 缩到 400x300 的框里。
    expect(img?.getAttribute("width")).toBe("400");
    shot?.click();
    await flush();
    expect(target.querySelector("dialog")).not.toBeNull();
    done();
  });

  it("keeps a big picture small until it is opened", async () => {
    const { target, done } = draw({
      text: withFile({
        url: "/files/a/big.png",
        name: "big.png",
        type: "image/png",
        size: 900000,
        preview: "/files/a/big.preview.png",
        width: 2000,
        height: 1000,
      }),
    });
    expect(target.querySelector(".shot img")?.getAttribute("src")).toBe("/files/a/big.preview.png");
    done();
  });

  it("plays sound and video in place", () => {
    const sound = draw({
      text: withFile({ url: "/files/a/take.mp3", name: "take.mp3", type: "audio/mpeg", size: 90000 }),
    });
    expect(sound.target.querySelector("audio.sound")).not.toBeNull();
    sound.done();

    const video = draw({
      text: withFile({
        url: "/files/a/ride.mp4",
        name: "ride.mp4",
        type: "video/mp4",
        size: 900000,
        width: 1280,
        height: 720,
      }),
    });
    expect(video.target.querySelector("video.clip")).not.toBeNull();
    video.done();
  });

  it("leaves everything else as a download", () => {
    const { target, done } = draw({
      text: withFile({ url: "/files/a/world.zip", name: "world.zip", type: "application/zip", size: 5242880 }),
    });
    expect(target.querySelector("a.file")?.getAttribute("href")).toBe("/files/a/world.zip");
    expect(target.textContent).toContain("5.0 MB");
    done();
  });
});
describe("read marks", () => {
  it("tells the server which message was seen last", async () => {
    const read: [number, number][] = [];
    const text = textOf({
      1: [
        { id: 1, channel: 1, author: "Kernel", text: "早", at: 1000 },
        { id: 7, channel: 1, author: "Ender", text: "来了", at: 2000 },
      ],
    });
    const { target, done } = draw({ text, onRead: (id: number, message: number) => read.push([id, message]) });
    await flush();
    expect(read).toContainEqual([1, 7]);
    expect(target.textContent).toContain("来了");
    done();
  });
});
describe("while someone types", () => {
  it("says who is typing and hides it when nobody is", () => {
    const text = textOf({ 1: [{ id: 1, channel: 1, author: "Kernel", text: "早", at: 1000 }] });
    const typing = draw({ text, typing: ["Ender"] });
    expect(typing.target.textContent).toContain("Ender 正在输入");
    typing.done();

    const quiet = draw({ text, typing: [] });
    expect(quiet.target.querySelector(".typing")).toBeNull();
    quiet.done();
  });

  it("tells the caller that I started typing", async () => {
    let told = 0;
    const { target, done } = draw({ onTyping: () => (told += 1) });
    const box = target.querySelector("textarea") as HTMLTextAreaElement;
    box.value = "在吗";
    box.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();
    expect(told).toBe(1);
    done();
  });
});

describe("links in a message", () => {
  it("draws the card the server made for it", () => {
    const text = textOf({ 1: [{ id: 5, channel: 1, author: "Kernel", text: "https://example.com/x", at: 1000 }] });
    const card = { url: "https://example.com/x", title: "一篇教程", description: "讲了怎么建刷石机", site: "example.com" };
    const { target, done } = draw({ text, previewOf: (id: number) => (id === 5 ? card : null), httpBase: "https://voice.example" });
    const link = target.querySelector("a.preview") as HTMLAnchorElement | null;
    expect(link?.getAttribute("href")).toBe("https://example.com/x");
    expect(target.textContent).toContain("一篇教程");
    expect(target.textContent).toContain("example.com");
    done();
  });
});