<script lang="ts">
  import Header from "./Header.svelte";
  import NavRail from "./NavRail.svelte";
  import PlayersView from "./PlayersView.svelte";
  import RoomGrid from "./RoomGrid.svelte";
  import SettingsView from "./SettingsView.svelte";
  import StatusRail from "./StatusRail.svelte";
  import VoiceBar from "./VoiceBar.svelte";
  import ChatView from "./ChatView.svelte";
  import Drawer from "./Drawer.svelte";
  import QuickSwitcher, { type Place } from "../components/QuickSwitcher.svelte";
  import SearchPanel from "../components/SearchPanel.svelte";
  import ThreadList from "../components/ThreadList.svelte";
  import { threadList, type Thread } from "../lib/threads";
  import { onMount, untrack } from "svelte";
  import { watchConnection, watchStatus, type LiveStatus } from "./live";
  import { joinRoom, leaveRooms } from "./rooms";
  import type { TextMessage, TextState } from "../lib/textChannels";
  import type { ConnectionInfo } from "../lib/connectionInfo.svelte";
  import {
    playerOfName,
    type ConnectionStats,
    type NavKey,
    type Person,
    type PersonAction,
    type RoomAction,
    type SguState,
    type VoiceRoom,
  } from "./types";

  // 麦克风和耳机的开关：接上真实语音后由外面给，界面自己不变。
  type Controls = {
    micOn: boolean;
    soundOn: boolean;
    onToggleMic: () => void;
    onToggleSound: () => void;
  };

  // 不叫 state：变量名 state 会让 $state 被当成 store 自动订阅。
  let {
    snapshot,
    rooms: roomsFromOutside,
    joinedRoomId: joinedFromOutside,
    stats,
    connectionInfo,
    controls,
    onJoin,
    onLeave,
    people: peopleFromOutside,
    canCreate,
    onCreate,
    chat,
    onSendText,
    onOpenChat,
    onUploadText,
    canWriteText,
    onReactText,
    onEditText,
    onPinText,
    onDeleteText,
    onCreateThread,
    onOpenThread,
    onSearchText,
    onPinsText,
    onOpenSettings,
    onOpenShortcuts,
    onOpenModeration,
    canModerateServer = false,
    personActions,
    roomActions,
    myId = null,
  }: {
    snapshot: SguState;
    // 接上真实语音时由外面给房间和当前房间；不给就自己管示例数据。
    rooms?: VoiceRoom[];
    joinedRoomId?: string | null;
    stats?: ConnectionStats | null;
    // 语音连接自己的统计，给顶栏那个小信号图标用。
    connectionInfo?: ConnectionInfo | null;
    controls?: Controls | null;
    onJoin?: (id: string) => void;
    onLeave?: () => void;
    // 所有频道里的人（玩家页用）；不给就按示例数据的房间推出来。
    people?: Person[];
    // 服务器说能建频道才显示「创建房间」。
    canCreate?: boolean;
    onCreate?: (name: string) => void;
    // 文字频道：接上真实连接才有，没接上时聊天入口显示占位。
    chat?: TextState | null;
    onSendText?: (
      channelId: number,
      body: string,
      extra?: { reply?: number; upload?: string },
    ) => void;
    onOpenChat?: (channelId: number) => void;
    // 传了才有「附件」按钮：把这个文件交上去，换回一个 key。
    onUploadText?: (file: File) => Promise<string>;
    // 没有发言权限的频道，输入框换成一句提示。
    canWriteText?: (channelId: number) => boolean;
    onReactText?: (messageId: number, emoji: string, on: boolean) => void;
    onEditText?: (messageId: number, body: string) => void;
    onPinText?: (messageId: number, on: boolean) => void;
    // 我在服务器上的用户 id：用来认出自己贴过的表情。
    myId?: number | null;
    // 删掉自己发的一条消息。
    onDeleteText?: (messageId: number) => void;
    // 从一条消息开一个话题，和打开某个话题。
    onCreateThread?: (messageId: number, name: string) => void;
    onOpenThread?: (threadId: number) => void;
    // 搜索消息、要某个频道的置顶列表。
    onSearchText?: (text: string) => void;
    onPinsText?: (channelId: number) => void;
    // 设置页里两扇门：上游的声音设置、键位表。
    onOpenSettings?: () => void;
    onOpenShortcuts?: () => void;
    onOpenModeration?: () => void;
    canModerateServer?: boolean;
    // 玩家页每一行能做的事（静音、踢出、封禁…）。
    personActions?: (session: number) => PersonAction[];
    // 房间卡片上的菜单（改名字、删掉）。
    roomActions?: (roomId: string) => RoomAction[];
  } = $props();

  let active = $state<NavKey>("voice");
  // 下面这几处只取挂载时的初值；之后的连接状态由 Shell 自己维护。
  let localJoined = $state<string | null>(untrack(() => snapshot.joinedRoomId));
  let localMic = $state(true);
  let localSound = $state(true);
  let unread = $state(0);
  // Ctrl+K 打开的那个跳转面板。
  let palette = $state(false);

  // 聊天页拉出来的抽屉：搜索、置顶、话题。
  let panel = $state<"search" | "pins" | "threads" | null>(null);
  let chatChannel = $state<number | null>(null);
  let openThread = $state<Thread | null>(null);
  // 从某条消息开话题时先问名字。
  let threadAsk = $state<number | null>(null);
  let threadName = $state("");

  // 后端配了游戏服才有真实人数；没有时页面用示例数据，方便做界面。
  let live = $state<LiveStatus | null>(null);
  // 延迟由浏览器自己测（每秒三次请求的空回答）；还没测出来时用示例数据。
  let measured = $state<ConnectionStats | null>(null);
  onMount(() => {
    // Ctrl+K / ⌘K，和上游一样。
    const hot = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        palette = true;
      }
    };
    window.addEventListener("keydown", hot);
    const stops: (() => void)[] = [];
    // 接上真实语音时延迟已经有人测了，不用再发一遍。
    if (!stats) stops.push(watchConnection((stats) => (measured = stats)));
    stops.push(watchStatus((status) => (live = status)));
    return () => {
      stops.forEach((stop) => stop());
      window.removeEventListener("keydown", hot);
    };
  });

  const connection = $derived(stats ?? measured ?? snapshot.connection);

  // 预计游戏延迟 = 你到这台服务器 + 这台服务器到游戏服。
  // 两段都是完整往返，中间过了同一个点，所以相加就是玩家那条路的期望值
  // （三角不等式：真实值不会比这个和更差）。其中一段没测到时不给数字。
  const predicted = $derived(
    typeof live?.rtt === "number" && connection.rtt > 0
      ? Math.round(connection.rtt + live.rtt)
      : null,
  );

  const server = $derived(
    live
      ? {
          ...snapshot.server,
          online: live.online,
          slots: live.slots,
          version: live.version ?? snapshot.server.version,
          // 服务器列表 ping 不报运行时间，不能拿示例数据顶着。
          uptime: undefined,
        }
      : snapshot.server,
  );

  // 服务器只报名字，位置和时长要装插件才有，所以不编。
  const players = $derived(live ? live.players.map((name, index) => playerOfName(name, index)) : snapshot.players);

  const meName = untrack(() => snapshot.me.name);

  // 抽屉里要用「这条在哪个频道」「这个频道有什么」。
  const textChannels = $derived(chat?.channels ?? []);
  const channelName = (id: number) => textChannels.find((channel) => channel.id === id)?.name ?? String(id);
  const whereOf = (message: TextMessage) => `#${channelName(message.channel)}`;
  const pinsOf = $derived(chatChannel === null ? [] : (chat?.pins[chatChannel] ?? []));
  const threadsOf = $derived.by(() => {
    const text = chat;
    if (!text || chatChannel === null) return [];
    return threadList(text.threads, text.archived[chatChannel]?.threads ?? [], chatChannel);
  });

  let localRooms = $state<VoiceRoom[]>(
    untrack(() => snapshot.rooms.map((room) => ({ ...room, members: [...room.members] }))),
  );

  const rooms = $derived(roomsFromOutside ?? localRooms);
  const joinedId = $derived(joinedFromOutside !== undefined ? joinedFromOutside : localJoined);
  const micOn = $derived(controls ? controls.micOn : localMic);
  const soundOn = $derived(controls ? controls.soundOn : localSound);

  const joined = $derived(rooms.find((room) => room.id === joinedId) ?? null);

  // 玩家页要的是「每个频道里有谁」：接上真实语音时由外面给，预览时从示例
  // 房间推出来（大厅里没人，所以只列房间）。
  const people = $derived(
    peopleFromOutside ??
      rooms.flatMap((room) =>
        room.members.map((name) => ({
          name,
          roomId: room.id,
          roomName: room.name,
          muted: false,
          deaf: false,
          session: -1,
          userId: null,
        })),
      ),
  );

  function join(id: string) {
    if (onJoin) {
      onJoin(id);
      return;
    }
    localRooms = joinRoom(localRooms, meName, id);
    localJoined = id;
  }

  function leave() {
    if (onLeave) {
      onLeave();
      return;
    }
    localRooms = leaveRooms(localRooms, meName);
    localJoined = null;
  }

  // 没接上服务器时建房间只加在页面上，免得按钮点了没反应。
  // 跳转面板里的入口：文字频道跳聊天，语音房间直接进。
  const places = $derived<Place[]>([
    ...(chat ? Object.values(chat.channels).map((channel) => ({
      key: `text-${channel.id}`,
      name: channel.name,
      kind: "text" as const,
      category: "文字频道",
      run: () => {
        openThread = null;
        chatChannel = channel.id;
        active = "chat";
      },
    })) : []),
    ...rooms.map((room) => ({
      key: `voice-${room.id}`,
      name: room.name,
      kind: "voice" as const,
      category: "语音房间",
      run: () => join(room.id),
    })),
  ]);

  function createRoom(name: string) {
    if (onCreate) {
      onCreate(name);
      return;
    }
    localRooms = [...localRooms, { id: `local-${localRooms.length + 1}`, name, members: [] }];
  }

  function toggleMic() {
    if (controls) controls.onToggleMic();
    else localMic = !localMic;
  }

  function toggleSound() {
    if (controls) controls.onToggleSound();
    else localSound = !localSound;
  }
</script>

<div class="sgu-app">
  <Header server={server} me={snapshot.me} info={connectionInfo ?? null} />

  <NavRail {active} roomCount={rooms.length} {unread} onSelect={(key) => (active = key)} />

  <main class="main">
    {#if active === "chat"}
      {#if chat && chat.available}
        <ChatView
          text={chat}
          onSend={(id, body, extra) => onSendText?.(id, body, extra)}
          onUpload={onUploadText}
          onOpen={onOpenChat}
          onUnread={(total) => (unread = total)}
          onReact={onReactText}
          onEdit={onEditText}
          onPin={onPinText}
          myName={meName}
          {myId}
          focusChannel={chatChannel}
          thread={openThread}
          onChannelChange={(id) => {
            chatChannel = id;
            if (panel === "pins") onPinsText?.(id);
            if (openThread !== null && openThread.channel !== id) openThread = null;
          }}
          onSearch={(query) => {
            onSearchText?.(query);
            panel = "search";
          }}
          onPins={(id) => {
            chatChannel = id;
            onPinsText?.(id);
            panel = "pins";
          }}
          onThreads={(id) => {
            chatChannel = id;
            panel = "threads";
          }}
          onDelete={onDeleteText}
          onThread={(id) => {
            threadAsk = id;
            threadName = "";
          }}
        />

        <Drawer
          open={panel !== null}
          title={panel === "pins" ? "置顶消息" : panel === "threads" ? "话题" : "搜索"}
          onClose={() => (panel = null)}
          bare
        >
          {#if panel === "search"}
            <SearchPanel
              text={chat.search?.text ?? ""}
              messages={chat.search?.messages ?? []}
              where={whereOf}
              canOpen={() => true}
              onOpen={(message) => {
                chatChannel = message.channel;
                panel = null;
              }}
              onClose={() => (panel = null)}
            />
          {:else if panel === "pins"}
            <SearchPanel
              text=""
              heading="置顶消息"
              label="置顶消息"
              none="这个频道还没有置顶消息。"
              messages={pinsOf}
              where={whereOf}
              canOpen={() => true}
              onOpen={(message) => {
                chatChannel = message.channel;
                panel = null;
              }}
              onClose={() => (panel = null)}
            />
          {:else if panel === "threads"}
            <ThreadList
              channel={chatChannel === null ? "" : channelName(chatChannel)}
              threads={threadsOf}
              more={false}
              unread={new Set<number>()}
              mentions={{}}
              onOpen={(thread) => {
                openThread = thread;
                chatChannel = thread.id;
                onOpenThread?.(thread.id);
                panel = null;
              }}
              onMore={() => {}}
              onClose={() => (panel = null)}
            />
          {/if}
        </Drawer>
      {:else if chat}
        <p class="todo">这个服务器没有开文字频道</p>
      {:else}
        <p class="todo">聊天还没接上</p>
      {/if}
    {:else if active === "players"}
      <PlayersView
        actions={personActions}
        {people}
        game={players}
        {meName}
        {joinedId}
        onJoin={onJoin ? join : undefined}
      />
    {:else if active === "settings"}
      <SettingsView
        {meName}
        {micOn}
        {soundOn}
        onToggleMic={toggleMic}
        onToggleSound={toggleSound}
        {onOpenSettings}
        {onOpenShortcuts}
        {onOpenModeration}
        canModerate={canModerateServer}
        onOpenPalette={() => (palette = true)}
      />
    {:else}
      <div class="head-row">
        <h2>语音房间</h2>
        <span>{rooms.length} 个</span>
      </div>

      <RoomGrid
        {rooms}
        {joinedId}
        {meName}
        onJoin={join}
        onCreate={createRoom}
        canCreate={canCreate ?? true}
        actions={roomActions}
      />
    {/if}
  </main>

  {#if palette}
  <QuickSwitcher {places} onClose={() => (palette = false)} />
{/if}

{#if threadAsk !== null}
    <button type="button" class="ask-backdrop" aria-label="取消" onclick={() => (threadAsk = null)}></button>
    <form
      class="ask"
      onsubmit={(event) => {
        event.preventDefault();
        const name = threadName.trim();
        if (name !== "" && threadAsk !== null) onCreateThread?.(threadAsk, name);
        threadAsk = null;
      }}
    >
      <h3>新话题</h3>
      <input bind:value={threadName} maxlength="60" placeholder="话题名字" />
      <div class="ask-acts">
        <button type="submit" class="ok">创建</button>
        <button type="button" onclick={() => (threadAsk = null)}>取消</button>
      </div>
    </form>
  {/if}

  <StatusRail
    connection={connection}
    server={server}
    activity={snapshot.activity}
    {predicted}
  />

  <VoiceBar
    roomName={joined?.name ?? null}
    count={joined?.members.length ?? 0}
    rtt={connection.rtt}
    {micOn}
    {soundOn}
    onToggleMic={toggleMic}
    onToggleSound={toggleSound}
    onLeave={leave}
  />
</div>

<style>
  .main {
    display: flex;
    grid-area: main;
    flex-direction: column;
    gap: 12px;
    min-height: 0;
    overflow-y: auto;
    padding: 26px 30px 30px;
  }

  .head-row {
    display: flex;
    align-items: baseline;
    gap: 10px;
  }

  .head-row h2 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }

  .head-row span {
    color: var(--sg-faint);
    font-size: 12.5px;
  }

  .todo {
    margin: 0;
    color: var(--sg-muted);
    font-size: 13px;
  }

  .ask-backdrop {
    position: fixed;
    inset: 0;
    z-index: 70;
    padding: 0;
    background: rgb(20 16 10 / 24%);
    border: 0;
  }

  .ask {
    position: fixed;
    top: 50%;
    left: 50%;
    z-index: 71;
    display: flex;
    flex-direction: column;
    gap: 10px;
    width: min(340px, 88vw);
    padding: 18px;
    background: var(--sg-card);
    border: 1px solid var(--sg-line);
    border-radius: 12px;
    box-shadow: 0 24px 60px rgb(44 38 29 / 18%);
    transform: translate(-50%, -50%);
  }

  .ask h3 {
    margin: 0;
    font-size: 14px;
    font-weight: 650;
  }

  .ask input {
    padding: 8px 10px;
    font: inherit;
    font-size: 13px;
    color: var(--sg-text);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 8px;
  }

  .ask input:focus {
    outline: none;
    border-color: var(--sg-green);
  }

  .ask-acts {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }

  .ask-acts button {
    padding: 6px 14px;
    font: inherit;
    font-size: 12.5px;
    color: var(--sg-muted);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 8px;
    cursor: pointer;
  }

  .ask-acts .ok {
    color: var(--sg-card);
    background: var(--sg-green);
    border-color: var(--sg-green);
  }
</style>
