// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { mount, unmount } from "svelte";
import type { Channel, ServerState, User } from "@kallisti/protocol";
import SguClient from "../src/sgu/SguClient.svelte";
import type { Connection } from "../src/lib/connection.svelte";

const channel = (id: number, name: string, parent: number | null, position = 0): Channel => ({
  id,
  parent,
  name,
  description: "",
  descriptionHash: null,
  position,
  temporary: false,
});

const user = (session: number, name: string, channelId: number, selfMute = false): User => ({
  session,
  name,
  channelId,
  userId: null,
  mute: false,
  deaf: false,
  suppress: false,
  selfMute,
  selfDeaf: false,
  prioritySpeaker: false,
  recording: false,
  texture: null,
  textureHash: null,
  comment: "",
  commentHash: null,
});

const serverOf = (users: User[]): ServerState => ({
  serverVersion: null,
  status: "synced",
  session: 7,
  welcomeText: "",
  error: null,
  rejectType: null,
  channels: Object.fromEntries(
    [
      channel(0, "服务器", null),
      channel(1, "大厅", 0),
      channel(2, "挖矿", 0, 1),
    ].map((c) => [c.id, c]),
  ),
  users: Object.fromEntries(users.map((u) => [u.session, u])),
  messages: [],
  privateCounts: {},
  notice: null,
  rootPermissions: null,
  channelPermissions: {},
  bans: null,
  registered: null,
});

// 这条连接只用到 state、info 和 send，其余都是客户端自己的事。
function fakeConnection(server: ServerState, sent: Uint8Array[]) {
  return {
    state: server,
    info: { rtt: 42, spread: 3, bad: 0.5 },
    send: (message: Uint8Array) => void sent.push(message),
  } as unknown as Connection;
}

function draw(server: ServerState, sent: Uint8Array[]) {
  const target = document.createElement("div");
  document.body.appendChild(target);
  const app = mount(SguClient, { target, props: { connection: fakeConnection(server, sent) } });
  return {
    target,
    done: () => {
      unmount(app);
      target.remove();
    },
  };
}

describe("the Chinese shell on a real connection", () => {
  it("shows the voice channels as rooms and the people in them", () => {
    const sent: Uint8Array[] = [];
    const { target, done } = draw(serverOf([user(7, "Kernel", 1)]), sent);
    expect(target.querySelector(".sgu-app")).not.toBeNull();
    expect(target.textContent).toContain("大厅");
    expect(target.textContent).toContain("挖矿");
    expect(target.textContent).toContain("你在这个房间");
    done();
  });

  it("walks into another room by moving the connection", () => {
    const sent: Uint8Array[] = [];
    const { target, done } = draw(serverOf([user(7, "Kernel", 1)]), sent);
    // 第一张是还在的大厅，第二张是挖矿。
    target.querySelectorAll<HTMLButtonElement>("button.room")[1]?.click();
    expect(sent).toHaveLength(1);
    expect(sent[0]?.length).toBeGreaterThan(0);
    done();
  });

  it("mutes for real instead of only in the page", () => {
    const sent: Uint8Array[] = [];
    const { target, done } = draw(serverOf([user(7, "Kernel", 1)]), sent);
    target.querySelector<HTMLButtonElement>("button.btn")?.click();
    expect(sent).toHaveLength(1);
    done();
  });
});
