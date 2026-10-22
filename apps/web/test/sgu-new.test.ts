// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { mount, unmount } from "svelte";
import type { Component } from "svelte";
import ChatView from "../src/sgu/ChatView.svelte";
import ChannelList from "../src/sgu/ChannelList.svelte";
import Shell from "../src/sgu/Shell.svelte";
import { readConfig } from "../src/sgu/live";
import { mockChat } from "../src/sgu/mockChat";
import { mockState } from "../src/sgu/mock";
import type { ChannelGroup, VoiceRoom } from "../src/sgu/types";
import type { TextState } from "../src/lib/textChannels";
import { SUGGESTIONS, suggestNames } from "../src/lib/mentions";
import { listKey, replaceTyped, typedWord } from "../src/lib/suggest";

function draw<Props extends Record<string, unknown>>(component: Component<Props>, props: Props) {
  const target = document.createElement("div");
  document.body.append(target);
  const instance = mount(component, { target, props });
  const done = () => {
    unmount(instance);
    target.remove();
  };
  return { target, done };
}

// Svelte 的更新在微任务里：等一拍再断言。
const settle = async () => new Promise((resolve) => setTimeout(resolve, 20));

const chat = () =>
  ({
  available: true,
  channels: [{ id: 1, name: "综合" }],
  loaded: { 1: true },
  messages: {
    1: [
      { id: 11, channel: 1, author: "Kernel", text: "<p>晚上八点开新矿洞。</p>", at: 1000 },
      { id: 12, channel: 1, author: "Ender", text: "<p>我先去。</p>", at: 2000 },
    ],
  },
}) as unknown as TextState;

// 补全用的是 lib/suggest 与 lib/mentions 那几段纯函数：jsdom 里派发的 input
// 事件到不了组件的 oninput（真浏览器里是好的），界面那半留给浏览器验收。
describe("mentioning someone", () => {
  const names = ["Kernel", "Ender"];

  it("finds the at sign being typed and who may be named", () => {
    expect(typedWord("喂 @en", 5)).toMatchObject({ kind: "@", query: "en" });
    expect(suggestNames(names, "en", "我")).toEqual(["Ender"]);
    expect(suggestNames(names, "", "我")).toContain("Kernel");
    expect(suggestNames(names, "", "我").length).toBeLessThanOrEqual(SUGGESTIONS);
  });

  it("finds the hash and the channel it may go to", () => {
    expect(typedWord("去 #建", 4)).toMatchObject({ kind: "#", query: "建" });
    expect(typedWord("没有标记", 4)).toBeNull();
    expect(typedWord("在 @", 3)).toMatchObject({ kind: "@", query: "" });
  });

  it("puts the pick where the word was and moves the caret after it", () => {
    const word = typedWord("喂 @en", 5)!;
    const next = replaceTyped("喂 @en", word.start, 5, "@Ender");
    expect(next.text).toBe("喂 @Ender ");
    expect(next.caret).toBe(next.text.length);
  });

  it("gives Enter and the arrows to the open list, and Esc closes it", () => {
    expect(listKey("ArrowDown", 3, 2)).toEqual({ active: 0 });
    expect(listKey("ArrowUp", 3, 0)).toEqual({ active: 2 });
    expect(listKey("Enter", 3, 0)).toBe("pick");
    expect(listKey("Escape", 3, 0)).toBe("close");
    expect(listKey("a", 3, 0)).toBeNull();
  });
});

describe("blocking someone", () => {
  it("hides what that person said", async () => {
    const { target, done } = draw(ChatView, { text: chat(), blocked: ["ender"], onSend: () => {} });
    await settle();
    expect(target.textContent).toContain("晚上八点开新矿洞");
    expect(target.textContent).not.toContain("我先去");
    done();
  });
});

describe("categories and sub rooms", () => {
  const groups: ChannelGroup[] = [
    { id: "0", name: "", rooms: [{ id: "1", name: "主城", members: [] }] },
    { id: "7", name: "公共区", rooms: [{ id: "8", name: "活动室", members: [] }] },
  ];

  const side = (made: [string, string][]) =>
    draw(ChannelList, {
      channels: [],
      unread: {},
      selected: null,
      onSelect: () => {},
      rooms: [{ id: "1", name: "主城", members: [] }] as VoiceRoom[],
      joinedId: null,
      meName: "我",
      onJoin: () => {},
      groups,
      onCreateIn: (parent: string, name: string) => made.push([parent, name]),
    });

  it("shows a group's name and creates a room inside it", async () => {
    const made: [string, string][] = [];
    const { target, done } = side(made);
    await settle();
    expect(target.textContent).toContain("公共区");

    target.querySelector<HTMLButtonElement>(".cat .add")!.click();
    await settle();

    const field = target.querySelector<HTMLInputElement>("input")!;
    field.value = "会议室";
    field.dispatchEvent(new Event("input", { bubbles: true }));
    await settle();
    target.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await settle();
    expect(made).toEqual([["7", "会议室"]]);
    done();
  });
});

describe("the sharing page", () => {
  it("reads the address from the server", async () => {
    const on = await readConfig(async () => new Response(JSON.stringify({ screenShare: "https://meet.example.org" })));
    expect(on.screenShare).toBe("https://meet.example.org");
    const off = await readConfig(async () => new Response("{}"));
    expect(off.screenShare).toBeNull();
  });

  it("explains how to turn it on when there is no page", async () => {
    const { target, done } = draw(Shell, { snapshot: mockState, chat: mockChat, screenShare: null });
    await settle();
    // 共享按钮只在语音里出现：先双击进一个房间。
    target.querySelector<HTMLElement>("button.voice")!.dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
    await settle();
    const share = [...target.querySelectorAll<HTMLButtonElement>("button")].find(
      (button) => button.textContent?.trim() === "共享屏幕",
    );
    expect(share).toBeDefined();
    share!.click();
    await settle();
    expect(target.textContent).toContain("SCREEN_SHARE_URL");
    done();
  });
});
