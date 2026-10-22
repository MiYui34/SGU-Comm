/** 界面用的数据形状：与后端无关，先由 mock 提供，接 RCON 后换成同样形状。 */

export type Dimension = "主世界" | "下界" | "末地";

/** 左侧栏的四个入口。 */
export type NavKey = "voice" | "chat" | "players" | "settings";

export interface ServerInfo {
  online: number;
  slots: number;
  /** 运行时间，已经是人话（"4 天 12 小时"）；服务器列表 ping 不报这个。 */
  uptime?: string;
  version: string;
}

export interface VoiceRoom {
  id: string;
  name: string;
  /** 房间里的人，按加入顺序。 */
  members: string[];
}

export interface Player {
  id: string;
  name: string;
  /** 游戏里的位置；服务器只报了名字时没有。 */
  dimension?: Dimension;
  /** 在线时长（分钟）；服务器只报了名字时没有。 */
  minutes?: number;
  /** 头像底色，取自名字。 */
  color: string;
}

/** 「你的延迟」：浏览器自己测出来的，不是玩家的游戏内 ping。 */
export interface ConnectionStats {
  rtt: number;
  jitter: number;
  loss: number;
}

export interface Activity {
  id: string;
  who: string;
  what: string;
  at: string;
}

export interface Me {
  name: string;
  initial: string;
  /** 是否已绑定 MC 账号。 */
  linked: boolean;
}

export interface SguState {
  server: ServerInfo;
  me: Me;
  rooms: VoiceRoom[];
  /** 用户当前所在的房间，没在语音里就是 null。 */
  joinedRoomId: string | null;
  players: Player[];
  connection: ConnectionStats;
  activity: Activity[];
}

/** 在线时长：分 → "4 小时 12 分"。 */
export function formatMinutes(minutes: number): string {
  if (minutes < 60) return `${minutes} 分钟`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${hours} 小时` : `${hours} 小时 ${rest} 分`;
}

/** 延迟好坏的一句话，跟着数字走，不要在每个卡片里各写一套。 */
export function qualityOf(rtt: number): string {
  if (rtt < 60) return "网络状况很好";
  if (rtt < 120) return "网络状况一般";
  return "网络不太稳定";
}

const avatarColors = ["#1f6b4a", "#b8442f", "#8a5a2b", "#3f5f8a", "#6b4a8a", "#2f6b6b"];

/** 头像底色：名字的哈希，同一个人每次都是同一个颜色。 */
export function playerColor(name: string): string {
  let hash = 0;
  for (const char of name) {
    hash = (hash * 31 + (char.codePointAt(0) ?? 0)) % 997;
  }
  return avatarColors[hash % avatarColors.length] ?? "#1f6b4a";
}

/** 服务器只报了名字：表格里就只用名字，不编造位置和时长。 */
export function playerOfName(name: string, index: number): Player {
  return { id: `live-${index}-${name}`, name, color: playerColor(name) };
}
