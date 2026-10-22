# SGU 定制说明

这个仓库是 [Kallisti](https://github.com/lambadalambda/kallisti) 的分支，给我们自己的
Minecraft 服务器用：语音还是 Mumble 那一套（浏览器里直接说话），上面加了一层中文界面、
游戏服务器状态和延迟测量。上游项目是 Unlicense（公有领域），这里同样如此。

## 加了什么

### 1. 中文界面外壳（`apps/web/src/sgu/`）

一套独立的中文界面：顶栏、左侧导航、语音房间卡片、在线玩家表、右侧状态栏、底部语音条。
它可以用两种方式打开：

- `https://你的域名/?ui=sgu`：登录后进入这套界面。房间是真实的语音频道，人是从连接里
  读出来的，进房间就是真的移动频道，麦克风按钮发的是真的 `UserState`。
- `/sgu.html`：不登录、用示例数据的预览页，用来调样式。

原来的 Discord 风格界面没有被删掉：地址不带 `?ui=sgu` 就还是它。

### 2. 游戏服务器状态（`GET /api/mc/status`）

用 Minecraft 的服务器列表 ping 拿在线人数、人数上限、版本和玩家名单，不用 RCON：

- RCON 密码不该出现在 Web 服务器上，也不该为了查个人数再开一个端口；
- `list` 命令的回复是本地化文本，解析起来很容易碎；
- ping 的 JSON 里本来就有 `players.online` / `players.max` / `sample[]` / `version.name`。

答案形如：

```json
{ "ok": true, "online": 3, "slots": 50, "version": "1.21.4",
  "players": ["Kernel", "Ender", "Buildar"], "rtt": 21, "at": 1792662423305 }
```

没配置时回 `{"ok": false, "configured": false}`；连不上时回
`{"ok": false, "configured": true, "error": "cannot reach the game server"}`，
前端两种都能降级显示。答案缓存 5 秒（失败也缓存），免得每次有人刷新都去敲游戏服。

**游戏服地址不会出现在前端。** `/api/mc/status` 只回人数、版本、名单和往返耗时；
地址只存在于服务端环境变量里，日志也只有服务端看得到。`mcstatus` 的测试里有一条专门
盯着这件事（`keeps the address to itself`）。

### 3. 延迟

浏览器开不了 TCP/UDP，所以拿不到「玩家到游戏服」这一段的真值。界面上的「预计游戏延迟」
是两段相加：

```
预计 = 你 ↔ 语音服务器  +  语音服务器 ↔ 游戏服
```

两段都过同一个中间点，按三角不等式，真实值不会比这个和更差；语音和游戏同机房时误差
一般只有几毫秒。任一段没测到就不显示这个数字。

- 第一段：浏览器对自己的 WebSocket 连接做 ping-pong（`GET /api/ping` 的空回答，
  每 5 秒一次，见 `apps/web/src/sgu/live.ts`）。
- 第二段：`mcstatus` 问游戏服的状态时顺带量的往返。

## 配置

| 变量 | 说明 |
| --- | --- |
| `MC_STATUS_ADDR` | 游戏服的 `host:port`，例如 `127.0.0.1:25565`。不填则状态卡片没有数据。 |

其余变量见 [configuration.md](configuration.md)。

## 开发

```sh
pnpm install                      # 根目录，一次就够
cd apps/web && pnpm dev           # http://localhost:5173/sgu.html 是预览页
```

后端：

```sh
cd server
go build ./...
go test ./internal/mcstatus/ ./internal/latency/ ./internal/config/
```

本地把两边连起来看真数据：

```sh
# 终端 1：假游戏服（3 人在线，回包延迟 20 ms）
cd <repo>
node dev/fake-mc.mjs               # MC_PORT=25566 MC_DELAY_MS=20

# 终端 2：Go 服务，把前端和状态接口一起伺服
cd server
MC_STATUS_ADDR=127.0.0.1:25566 ALLOWED_ORIGINS=* LISTEN_PORT=8099 \
  WEB_DIR=../apps/web/dist go run ./cmd/server
```

然后开 `http://127.0.0.1:8099/?ui=sgu`。前端开发时用
`cd apps/web && SGU_API=http://127.0.0.1:8099 pnpm dev`，vite 会把 `/api/mc` 转过去。

## 检查

```sh
cd apps/web
pnpm typecheck        # svelte-check，0 错 0 警
npx vitest run        # 全量：112 个文件 1000+ 用例
pnpm build            # 产物含 index.html 和 sgu.html
```

## 还没做完

- 聊天入口接了文字频道（读最近消息、发消息），但还没有上传附件、回复、表情回应、
  已读和未读标记；
- 「设置」入口还是占位；切换回原界面要手动改地址；
- TPS、维度、在线时长要装插件走 RCON，现在只有 ping 给的人数和名单；
- 账号与 Minecraft 玩家绑定（六位码 → UUID → 头像用皮肤）还没做；
- 上游 12841 行英文界面还没中文化 —— 现在中文只在外壳这一层。
