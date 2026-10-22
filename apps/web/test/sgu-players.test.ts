// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { mount, unmount } from "svelte";
import type { Component } from "svelte";
import PlayersView from "../src/sgu/PlayersView.svelte";
import RoomGrid from "../src/sgu/RoomGrid.svelte";
import SettingsView from "../src/sgu/SettingsView.svelte";
import { playerOfName, type Person, type VoiceRoom } from "../src/sgu/types";

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

const people: Person[] = [
  { name: "Kernel", roomId: "0", roomName: "大厅", muted: false, deaf: false, session: 2, userId: 2 },
  { name: "Ender", roomId: "1", roomName: "主城", muted: true, deaf: false, session: 3, userId: 3 },
  { name: "我", roomId: "1", roomName: "主城", muted: false, deaf: true, session: 1, userId: 1 },
];

const rooms: VoiceRoom[] = [
  { id: "1", name: "主城", members: ["Ender", "我"] },
  { id: "2", name: "挖矿", members: [] },
];

describe("players page", () => {
  it("lists everyone room by room, lobby included", () => {
    const { target, done } = draw(PlayersView, { people, game: [], meName: "我" });
    expect(target.textContent).toContain("大厅");
    expect(target.textContent).toContain("主城");
    expect(target.textContent).toContain("Kernel");
    expect(target.textContent).toContain("Ender");
    expect(target.textContent).toContain("3 人");
    done();
  });

  it("marks me, a closed microphone and muted headphones", () => {
    const { target, done } = draw(PlayersView, { people, game: [], meName: "我" });
    expect(target.textContent).toContain("你");
    expect(target.textContent).toContain("麦克风已关");
    expect(target.textContent).toContain("耳机静音");
    done();
  });

  it("shows who is in the game", () => {
    const { target, done } = draw(PlayersView, {
      people,
      game: [playerOfName("Kernel", 0), playerOfName("Buildar", 1)],
      meName: "我",
    });
    expect(target.textContent).toContain("游戏里");
    expect(target.textContent).toContain("Buildar");
    done();
  });

  it("offers to join a room, but not the lobby or the room I am in", () => {
    const joined: string[] = [];
    const more: Person[] = [
      ...people,
      { name: "Mia", roomId: "3", roomName: "挖矿", muted: false, deaf: false, session: 4, userId: 4 },
    ];
    const { target, done } = draw(PlayersView, {
      people: more,
      game: [],
      meName: "我",
      joinedId: "1",
      onJoin: (id: string) => joined.push(id),
    });
    const buttons = [...target.querySelectorAll("button")];
    // 只有「挖矿」有入口：大厅不能进，自己所在的主城也不用进。
    expect(buttons.map((button) => button.textContent?.trim())).toEqual(["进入"]);
    buttons[0]?.click();
    expect(joined).toEqual(["3"]);
    done();
  });

  it("says so when nobody is around", () => {
    const { target, done } = draw(PlayersView, { people: [], game: [], meName: "我" });
    expect(target.textContent).toContain("现在没有人在语音里");
    done();
  });
});

describe("settings page", () => {
  it("turns the microphone and the speakers on and off", () => {
    const calls: string[] = [];
    const { target, done } = draw(SettingsView, {
      meName: "Kernel",
      micOn: true,
      soundOn: false,
      onToggleMic: () => calls.push("mic"),
      onToggleSound: () => calls.push("sound"),
    });
    const switches = [...target.querySelectorAll('[role="switch"]')];
    expect(switches.map((sw) => sw.getAttribute("aria-checked"))).toEqual(["true", "false"]);
    switches[0]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    switches[1]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    expect(calls).toEqual(["mic", "sound"]);
    expect(target.textContent).toContain("Kernel");
    expect(target.textContent).toContain("中文");
    done();
  });
});

describe("creating a room", () => {
  it("asks for a name and hands it over", async () => {
    const created: string[] = [];
    const { target, done } = draw(RoomGrid, {
      rooms,
      joinedId: null,
      meName: "我",
      onJoin: () => {},
      onCreate: (name: string) => created.push(name),
    });

    target.querySelector<HTMLButtonElement>(".create")?.click();
    await new Promise((resolve) => setTimeout(resolve, 10));

    const input = target.querySelector<HTMLInputElement>(".create input");
    expect(input).not.toBeNull();
    if (input) {
      input.value = "音乐";
      input.dispatchEvent(new Event("input", { bubbles: true }));
    }
    // jsdom 不会因为点了提交按钮就真的提交表单，直接发 submit。
    target
      .querySelector("form")
      ?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

    expect(created).toEqual(["音乐"]);

    // 关掉表单也是下一帧的事。
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(target.querySelector(".create input")).toBeNull();
    done();
  });

  it("hides the way in when the server does not allow it", () => {
    const { target, done } = draw(RoomGrid, {
      rooms,
      joinedId: null,
      meName: "我",
      onJoin: () => {},
      canCreate: false,
    });
    expect(target.querySelector(".create")).toBeNull();
    done();
  });
});

describe("what I may do to someone", () => {
  it("opens a menu on a row and runs what was picked", async () => {
    const ran: string[] = [];
    const { target, done } = draw(PlayersView, {
      people,
      game: [],
      meName: "我",
      actions: (session: number) => [
        { key: "mute", label: "静音", run: () => ran.push(`mute ${session}`) },
        { key: "kick", label: "踢出", run: () => ran.push(`kick ${session}`) },
      ],
    });
    const more = target.querySelector<HTMLButtonElement>("button.more");
    if (more === null) throw new Error("no menu button");
    more.click();
    await new Promise((resolve) => setTimeout(resolve, 10));
    const items = [...target.querySelectorAll<HTMLButtonElement>(".menu button")];
    expect(items.map((item) => item.textContent?.trim())).toEqual(["静音", "踢出"]);
    items[1]?.click();
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(ran).toEqual(["kick 2"]);
    done();
  });

  it("keeps the menu away when there is nothing to do", () => {
    const { target, done } = draw(PlayersView, { people, game: [], meName: "我", actions: () => [] });
    expect(target.querySelector("button.more")).toBeNull();
    done();
  });
});

describe("the settings page", () => {
  it("opens the full voice settings and the key list", async () => {
    const opened: string[] = [];
    const { target, done } = draw(SettingsView, {
      meName: "我",
      micOn: true,
      soundOn: true,
      onToggleMic: () => {},
      onToggleSound: () => {},
      onOpenSettings: () => opened.push("settings"),
      onOpenShortcuts: () => opened.push("keys"),
    });
    const buttons = [...target.querySelectorAll<HTMLButtonElement>("button.more")];
    for (const button of buttons) button.click();
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(opened).toEqual(["settings", "keys"]);
    done();
  });

  it("offers the server management only to those who may use it", () => {
    const plain = draw(SettingsView, {
      meName: "我",
      micOn: true,
      soundOn: true,
      onToggleMic: () => {},
      onToggleSound: () => {},
    });
    expect(plain.target.textContent).not.toContain("服务器管理");
    plain.done();

    const staff = draw(SettingsView, {
      meName: "我",
      micOn: true,
      soundOn: true,
      onToggleMic: () => {},
      onToggleSound: () => {},
      canModerate: true,
      onOpenModeration: () => {},
    });
    expect(staff.target.textContent).toContain("服务器管理");
    staff.done();
  });
});