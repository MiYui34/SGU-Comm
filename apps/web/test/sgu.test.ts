// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { mount, unmount } from "svelte";
import type { Component } from "svelte";
import Header from "../src/sgu/Header.svelte";
import RoomGrid from "../src/sgu/RoomGrid.svelte";
import Shell from "../src/sgu/Shell.svelte";
import StatusRail from "../src/sgu/StatusRail.svelte";
import { mockState } from "../src/sgu/mock";

// jsdom 没有滚动，只有真浏览器才有；跳转面板会用到它。
window.HTMLElement.prototype.scrollIntoView = () => {};

// jsdom 的 <dialog> 还没有模态方法，跳转面板一开就调它。
HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) {
  this.open = true;
};
HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) {
  this.open = false;
  this.dispatchEvent(new Event("close"));
};
import { joinRoom, leaveRooms } from "../src/sgu/rooms";
import { probe, readStatus, summarise } from "../src/sgu/live";
import { formatMinutes, qualityOf, playerOfName, type VoiceRoom } from "../src/sgu/types";

const rooms: VoiceRoom[] = [
  { id: "a", name: "主城大厅", members: ["Kernel", "我"] },
  { id: "b", name: "挖矿队", members: ["Ender"] },
  { id: "c", name: "建筑组", members: [] },
];

describe("rooms", () => {
  it("moves me to the target room and out of the others", () => {
    const [a, b, c] = joinRoom(rooms, "我", "b");
    expect(a?.members).toEqual(["Kernel"]);
    expect(b?.members).toEqual(["Ender", "我"]);
    expect(c?.members).toEqual([]);
  });

  it("does not put me in twice when I am there already", () => {
    const next = joinRoom(joinRoom(rooms, "我", "b"), "我", "b");
    expect(next[1]?.members).toEqual(["Ender", "我"]);
  });

  it("leaves me out of every room", () => {
    expect(leaveRooms(rooms, "我").map((room) => room.members)).toEqual([["Kernel"], ["Ender"], []]);
  });

  it("keeps the input untouched", () => {
    joinRoom(rooms, "我", "b");
    leaveRooms(rooms, "我");
    expect(rooms[0]?.members).toEqual(["Kernel", "我"]);
  });
});

describe("formatting", () => {
  it("says minutes, hours and both", () => {
    expect(formatMinutes(9)).toBe("9 分钟");
    expect(formatMinutes(180)).toBe("3 小时");
    expect(formatMinutes(252)).toBe("4 小时 12 分");
  });

  it("describes the round trip time", () => {
    expect(qualityOf(28)).toBe("网络状况很好");
    expect(qualityOf(90)).toBe("网络状况一般");
    expect(qualityOf(180)).toBe("网络不太稳定");
  });

  it("gives a name a stable color", () => {
    expect(playerOfName("Kernel", 0).color).toBe(playerOfName("Kernel", 3).color);
    expect(playerOfName("Kernel", 0).id).not.toBe(playerOfName("Ender", 0).id);
  });
});

describe("live status", () => {
  function answer(body: unknown, ok = true) {
    return (async () => ({ ok, json: async () => body })) as unknown as typeof fetch;
  }

  it("reads the counts the backend answers", async () => {
    const status = await readStatus(answer({ ok: true, online: 3, slots: 20, version: "1.21.4", players: ["A", 2] }));
    expect(status).toEqual({ online: 3, slots: 20, version: "1.21.4", players: ["A"] });
  });

  it("ignores an answer for a server with no Minecraft configured", async () => {
    expect(await readStatus(answer({ ok: false, configured: false }))).toBeNull();
  });

  it("survives a network error", async () => {
    const offline = (async () => {
      throw new Error("offline");
    }) as unknown as typeof fetch;
    expect(await readStatus(offline)).toBeNull();
  });
});

// 预计游戏延迟要把两段加起来：浏览器测的那段 + 服务器报的那段。
describe("predicted game delay", () => {
  it("takes the round trip the server reports", async () => {
    const answer = (async () => ({
      ok: true,
      json: async () => ({ ok: true, online: 1, slots: 20, players: [], rtt: 12 }),
    })) as unknown as typeof fetch;
    expect((await readStatus(answer))?.rtt).toBe(12);
  });

  it("has no number when the server does not report one", async () => {
    const answer = (async () => ({
      ok: true,
      json: async () => ({ ok: true, online: 1, slots: 20, players: [] }),
    })) as unknown as typeof fetch;
    expect((await readStatus(answer))?.rtt).toBeUndefined();
  });
});

// 「你的延迟」是浏览器自己量出来的：请求出去再回来用掉的时间。
describe("probe", () => {
  function clock(...times: number[]) {
    let index = 0;
    return () => times[index++] ?? 0;
  }

  it("times the round trip", async () => {
    const empty = (async () => new Response(null, { status: 204 })) as unknown as typeof fetch;
    expect(await probe(empty, clock(1000, 1028))).toBe(28);
  });

  it("counts a failed answer as a lost sample", async () => {
    const refused = (async () => new Response(null, { status: 502 })) as unknown as typeof fetch;
    expect(await probe(refused, clock(0, 5))).toBeNull();
  });

  it("survives a network error", async () => {
    const offline = (async () => {
      throw new Error("offline");
    }) as unknown as typeof fetch;
    expect(await probe(offline, clock(0, 5))).toBeNull();
  });

  it("calls the probe endpoint without caching", async () => {
    const seen: (string | undefined)[] = [];
    const empty = (async (url: string, init: RequestInit) => {
      seen.push(`${url} ${init.cache ?? ""}`);
      return new Response(null, { status: 204 });
    }) as unknown as typeof fetch;
    await probe(empty, clock(0, 3));
    expect(seen).toEqual(["/api/ping no-store"]);
  });
});

describe("connection summary", () => {
  it("averages the delay, the jitter and counts the losses", () => {
    expect(summarise([30, 34, null, 32])).toEqual({ rtt: 32, jitter: 3, loss: 25 });
  });

  it("reports everything lost when nothing came back", () => {
    expect(summarise([null, null])).toEqual({ rtt: 0, jitter: 0, loss: 100 });
  });

  it("has no jitter with a single sample", () => {
    expect(summarise([40])).toEqual({ rtt: 40, jitter: 0, loss: 0 });
  });
});

// 冒烟测试：模板能否编译、组件之间的属性传递、文案是否真的渲染出来。
describe("components", () => {
  function draw<Props extends Record<string, unknown>>(component: Component<Props>, props: Props) {
    const target = document.createElement("div");
    document.body.appendChild(target);
    const app = mount(component, { target, props });
    return {
      target,
      done: () => {
        unmount(app);
        target.remove();
      },
    };
  }

  it("adds the two halves of the delay into a guess about the game", () => {
    const { target, done } = draw(StatusRail, {
      connection: { rtt: 28, jitter: 3.2, loss: 0 },
      server: mockState.server,
      activity: mockState.activity,
      predicted: 58,
    });
    expect(target.textContent).toContain("预计游戏延迟");
    expect(target.textContent).toContain("58 ms");
    done();
  });

  it("shows no guess while one half is missing", () => {
    const { target, done } = draw(StatusRail, {
      connection: { rtt: 28, jitter: 3.2, loss: 0 },
      server: mockState.server,
      activity: mockState.activity,
      predicted: null,
    });
    expect(target.textContent).not.toContain("预计游戏延迟");
    done();
  });

  it("shows the server name and the counts in the header", () => {
    const { target, done } = draw(Header, {
      server: mockState.server,
      me: mockState.me,
      active: "chat",
      roomCount: 3,
      unread: 2,
      onSelect: () => {},
    });
    expect(target.textContent).toContain("SGU 生存服");
    expect(target.textContent).toContain("12");
    expect(target.textContent).toContain("50 在线");
    done();
  });

  it("marks the room I am in and shows who is there", () => {
    let joined: string | null = null;
    const { target, done } = draw(RoomGrid, {
      rooms: mockState.rooms,
      joinedId: "lobby",
      meName: mockState.me.name,
      onJoin: (id: string) => (joined = id),
      onCreate: () => {},
    });
    expect(target.textContent).toContain("主城大厅");
    expect(target.textContent).toContain("你在这个房间");
    target.querySelectorAll("button")[1]?.click();
    expect(joined).toBe("mining");
    done();
  });

  it("renders the whole page with the mock data", () => {
    const { target, done } = draw(Shell, { snapshot: mockState });
    expect(target.querySelector(".sgu-app")).not.toBeNull();
    expect(target.textContent).toContain("你的延迟");
    expect(target.textContent).toContain("28");
    expect(target.textContent).toContain("最近动态");
    expect(target.textContent).toContain("语音房间");
    expect(target.textContent).toContain("4 天 12 小时");
    done();
  });

  it("shows the counts the game server answers", async () => {
    const original = globalThis.fetch;
    globalThis.fetch = (async () => ({
      ok: true,
      json: async () => ({ ok: true, online: 7, slots: 20, version: "1.21.5", players: ["LiveOnly"] }),
    })) as unknown as typeof fetch;
    try {
      const { target, done } = draw(Shell, { snapshot: mockState });
      await settle(() => (target.textContent ?? "").includes("7 / 20"));
      expect(target.textContent).toContain("7 / 20");
      expect(target.textContent).not.toContain("12 / 50");

      // 游戏里的人名在玩家页上，不在语音页。
      const nav = [...target.querySelectorAll<HTMLButtonElement>("nav button")];
      nav[2]?.click();
      await settle(() => (target.textContent ?? "").includes("LiveOnly"));
      expect(target.textContent).toContain("LiveOnly");
      done();
    } finally {
      globalThis.fetch = original;
    }
  });


});

// Svelte 的更新在微任务里，等界面自己刷出来再断言。
async function settle(done: () => boolean) {
  for (let i = 0; i < 100; i += 1) {
    if (done()) return;
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
}
