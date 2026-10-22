import type { SguState } from "./types";

/**
 * 假数据：先把界面跑起来，接上 RCON 与语音状态后整块替换。
 * 形状和 lib/type 里的 SguState 一致，接真实数据时组件不用改。
 */
export const mockState: SguState = {
  server: { online: 12, slots: 50, uptime: "4 天 12 小时", version: "1.21.4" },
  me: { name: "Steve_SGU", initial: "我", linked: false },
  joinedRoomId: "lobby",
  rooms: [
    { id: "lobby", name: "主城大厅", members: ["Kernel", "小樱", "Alex_Craft", "Steve_SGU"] },
    { id: "mining", name: "挖矿队", members: ["Ender", "Steve_CN", "PickaxePro"] },
    { id: "build", name: "建筑组", members: [] },
    { id: "music", name: "音乐房", members: ["点歌Bot", "Panda"] },
    { id: "admin", name: "管理组", members: ["Admin_Zero"] },
  ],
  players: [
    { id: "1", name: "Alex_Craft", dimension: "主世界", minutes: 252, color: "#3f8f6b" },
    { id: "2", name: "小樱", dimension: "下界", minutes: 155, color: "#c46a4a" },
    { id: "3", name: "Kernel", dimension: "主世界", minutes: 78, color: "#4a7bd4" },
    { id: "4", name: "Panda", dimension: "主世界", minutes: 185, color: "#a04a8a" },
    { id: "5", name: "Ender", dimension: "末地", minutes: 42, color: "#8a6fd0" },
    { id: "6", name: "Steve_CN", dimension: "主世界", minutes: 9, color: "#4aa3c4" },
  ],
  connection: { rtt: 28, jitter: 3.2, loss: 0 },
  activity: [
    { id: "a1", who: "Alex_Craft", what: "进入大厅", at: "刚刚" },
    { id: "a2", who: "Tech_Guy", what: "退出游戏", at: "2 分钟前" },
    { id: "a3", who: "小樱", what: "去了下界", at: "9 分钟前" },
  ],
};
