import type { TextState } from "../lib/textChannels";

// 预览页用的聊天数据：不连服务器时也能看到附件、回复、表情、编辑和置顶长什么样。
const now = Date.parse("2026-10-22T19:40:00+08:00");

export const mockChat = {
  available: true,
  channels: [
    { id: 1, name: "综合" },
    { id: 2, name: "建筑" },
  ],
  categories: [],
  messages: {
    1: [
      {
        id: 101,
        channel: 1,
        author: "Kernel",
        text: "晚上八点开新矿洞，带足火把。",
        at: now - 26 * 60_000,
        pinned: now - 20 * 60_000,
        reactions: [
          { emoji: "👍", users: ["Ender", "Steve_SGU", "Buildar"], ids: [11, 1, 12] },
          { emoji: "🔥", users: ["Ender"], ids: [11] },
        ],
      },
      {
        id: 102,
        channel: 1,
        author: "Ender",
        text: "我这边有多的圆石，谁要？",
        at: now - 18 * 60_000,
        reactions: [{ emoji: "👀", users: ["Kernel"], ids: [10] }],
      },
      {
        id: 103,
        channel: 1,
        author: "Steve_SGU",
        text: "我要一箱，谢了。",
        at: now - 16 * 60_000,
        edited: now - 15 * 60_000,
        reply: { id: 102, author: "Ender", text: "我这边有多的圆石，谁要？" },
      },
      {
        id: 104,
        channel: 1,
        author: "Ender",
        text: "图我放这里，照着挖就行。",
        at: now - 9 * 60_000,
        // 张示例图，放在 public 里；/files/ 是上传目录，别撞上。
        upload: {
          url: "/mock-mine.png",
          name: "矿洞路线.png",
          size: 9_466,
          type: "image/png",
          width: 320,
          height: 200,
        },
      },
    ],
    2: [
      {
        id: 201,
        channel: 2,
        author: "Buildar",
        text: "钟楼的地基打好了，明天上木料。",
        at: now - 40 * 60_000,
      },
    ],
  },
  loaded: { 1: true, 2: true },
  uploadMax: 8 * 1024 * 1024,
} as unknown as TextState;
