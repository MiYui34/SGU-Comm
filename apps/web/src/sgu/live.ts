// 服务器状态由后端去问游戏服（服务器列表 ping），浏览器只读这份结果：
// 游戏服地址与凭据都不进前端。
import type { ConnectionStats } from "./types";

export type LiveStatus = {
  online: number;
  slots: number;
  version?: string;
  players: string[];
  /** 服务器到游戏服的往返（毫秒）：预计游戏延迟的另一半。 */
  rtt?: number;
};

type Answer = {
  ok?: boolean;
  online?: number;
  slots?: number;
  version?: string;
  players?: unknown;
  rtt?: unknown;
};

// 没配置游戏服时后端回 { ok: false, configured: false }：这里给 null，
// 页面继续用示例数据。
export async function readStatus(fetcher: typeof fetch = fetch): Promise<LiveStatus | null> {
  try {
    const response = await fetcher("/api/mc/status", { headers: { Accept: "application/json" } });
    if (!response.ok) return null;
    const answer = (await response.json()) as Answer;
    if (answer.ok !== true || typeof answer.online !== "number" || typeof answer.slots !== "number") return null;
    const players = Array.isArray(answer.players) ? answer.players.filter((name): name is string => typeof name === "string") : [];
    const rtt = typeof answer.rtt === "number" ? answer.rtt : undefined;
    return { online: answer.online, slots: answer.slots, version: answer.version, players, rtt };
  } catch {
    return null;
  }
}

// 每 15 秒看一次；前一次回来之后才排下一次，避免请求堆起来。
// 返回停止的函数，组件卸载时要用。
export function watchStatus(onChange: (status: LiveStatus | null) => void, every = 15000): () => void {
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const tick = async () => {
    const status = await readStatus();
    if (stopped) return;
    onChange(status);
    timer = setTimeout(tick, every);
  };
  void tick();
  return () => {
    stopped = true;
    if (timer !== undefined) clearTimeout(timer);
  };
}

// 一个样本：请求出去再回来用掉的时间（毫秒）。失败给 null。
//
// 这是浏览器到服务器自己的往返，不是玩家的游戏内 ping：游戏服的延迟
// 浏览器测不到。
export async function probe(
  fetcher: typeof fetch = fetch,
  now: () => number = () => performance.now(),
): Promise<number | null> {
  const started = now();
  try {
    const response = await fetcher("/api/ping", { cache: "no-store" });
    if (!response.ok) return null;
    return Math.max(0, Math.round(now() - started));
  } catch {
    return null;
  }
}

// 样本窗口 → 延迟、抖动、丢包。抖动取相邻两次成功样本差值的平均。
export function summarise(samples: (number | null)[]): ConnectionStats {
  const ok = samples.filter((rtt): rtt is number => rtt !== null);
  if (ok.length === 0) return { rtt: 0, jitter: 0, loss: 100 };

  const total = ok.reduce((sum, rtt) => sum + rtt, 0);
  let jitter = 0;
  for (let index = 1; index < ok.length; index += 1) {
    jitter += Math.abs((ok[index] ?? 0) - (ok[index - 1] ?? 0));
  }

  const pairs = ok.length - 1;
  const loss = Math.round(((samples.length - ok.length) / samples.length) * 1000) / 10;

  return {
    rtt: Math.round((total / ok.length) * 10) / 10,
    jitter: pairs > 0 ? Math.round((jitter / pairs) * 10) / 10 : 0,
    loss,
  };
}

// 每 3 秒测一次，只留最近 8 个样本：数字会动，但不会跳。
//
// 第一个样本带着建连接的开销（握手、DNS），它不代表网络延迟，所以丢掉。
export function watchConnection(onChange: (stats: ConnectionStats) => void, every = 3000, size = 8): () => void {
  let stopped = false;
  let first = true;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const samples: (number | null)[] = [];
  const tick = async () => {
    const rtt = await probe();
    if (stopped) return;
    if (first) {
      first = false;
    } else {
      samples.push(rtt);
      if (samples.length > size) samples.shift();
      onChange(summarise(samples));
    }
    timer = setTimeout(tick, every);
  };
  void tick();
  return () => {
    stopped = true;
    if (timer !== undefined) clearTimeout(timer);
  };
}
