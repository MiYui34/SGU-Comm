<script lang="ts">
  import Header from "./Header.svelte";
  import ChannelList from "./ChannelList.svelte";
  import Ask from "./Ask.svelte";
  import PlayersView from "./PlayersView.svelte";
  import RoomGrid from "./RoomGrid.svelte";
  import SettingsView from "./SettingsView.svelte";
  import StatusRail from "./StatusRail.svelte";
  import VoiceBar from "./VoiceBar.svelte";
  import ChatView from "./ChatView.svelte";
  import Drawer from "./Drawer.svelte";
  import SearchPanel from "../components/SearchPanel.svelte";
  import ThreadList from "../components/ThreadList.svelte";
  import { threadList, type Thread } from "../lib/threads";
  import type { LinkPreview } from "../lib/linkPreview";
  import { typingNames } from "../lib/typing";
  import { onMount, untrack } from "svelte";
  import { watchConnection, watchStatus, type LiveStatus } from "./live";
  import { joinRoom, leaveRooms } from "./rooms";
  import type { TextMessage, TextState } from "../lib/textChannels";
  import type { ConnectionInfo } from "../lib/connectionInfo.svelte";
  import {
    playerOfName,
    type ChannelGroup,
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
    groups,
    joinedRoomId: joinedFromOutside,
    stats,
    connectionInfo,
    controls,
    onJoin,
    onLeave,
    people: peopleFromOutside,
    canCreate,
    onCreate,
    onCreateIn,
    chat,
    onSendText,
    onOpenChat,
    onReadText,
    screenShare,
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
    typers,
    typingNow,
    onTypingText,
    previewOf,
    httpBase = "",
    directs = [],
    onOpenSettings,
    onOpenShortcuts,
    onOpenModeration,
    canModerateServer = false,
    personActions,
    blocked = [],
    roomActions,
    myId = null,
  }: {
    snapshot: SguState;
    // 接上真实语音时由外面给房间和当前房间；不给就自己管示例数据。
    rooms?: VoiceRoom[];
    // 带分类的房间分组（接上真实语音时由外面算好）。
    groups?: ChannelGroup[];
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
    // 在某个房间（有了子房间就成了分类）底下建新的。
    onCreateIn?: (parentId: string, name: string) => void;
    // 文字频道：接上真实连接才有，没接上时聊天入口显示占位。
    chat?: TextState | null;
    onSendText?: (
      channelId: number,
      body: string,
      extra?: { reply?: number; upload?: string },
    ) => void;
    onOpenChat?: (channelId: number) => void;
    // 读到哪条了，交给服务器记下来。
    onReadText?: (channelId: number, messageId: number) => void;
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
    /** 共享页的地址（自建 Jitsi Meet 之类）；null 表示这个部署没配。 */
    screenShare?: string | null;
    // 谁在哪个频道打字（上游 lib/typing 的结构），和它算过期用的时刻。
    typers?: Readonly<Record<number, Readonly<Record<string, number>>>>;
    typingNow?: number;
    // 我开始打字了，报给服务器。
    onTypingText?: (channelId: number) => void;
    // 消息里的链接卡：服务器抓好的结果按消息 id 查。
    previewOf?: (messageId: number) => LinkPreview | null;
    // 卡片上的图在服务器上，要拼绝对地址。
    httpBase?: string;
    // 私信：和某个人的一对一聊天，跟文字频道一样列在左栏。
    directs?: { id: number; name: string }[];
    // 设置页里两扇门：上游的声音设置、键位表。
    onOpenSettings?: () => void;
    onOpenShortcuts?: () => void;
    onOpenModeration?: () => void;
    canModerateServer?: boolean;
    // 玩家页每一行能做的事（静音、踢出、封禁…）。
    personActions?: (session: number) => PersonAction[];
    // 我屏蔽了的人：聊天里不显示他们说的话。
    blocked?: readonly string[];
    // 房间卡片上的菜单（改名字、删掉）。
    roomActions?: (roomId: string) => RoomAction[];
  } = $props();

  // 默认落在聊天上：中间是消息，左边是频道。
  let active = $state<NavKey>("chat");
  // 下面这几处只取挂载时的初值；之后的连接状态由 Shell 自己维护。
  let localJoined = $state<string | null>(untrack(() => snapshot.joinedRoomId));
  let localMic = $state(true);
  let localSound = $state(true);
  let unread = $state(0);
  let sharing = $state(false);
  // 每个频道各剩几条没看，左边栏的绿点用它。
  let unreadOf = $state<Record<number, number>>({});

  // 聊天页拉出来的抽屉：搜索、置顶、话题。
  let panel = $state<"search" | "pins" | "threads" | null>(null);
  let chatChannel = $state<number | null>(null);
  let openThread = $state<Thread | null>(null);
  // 从某条消息开话题时先问名字。
  let threadAsk = $state<number | null>(null);

  // 后端配了游戏服才有真实人数；没有时页面用示例数据，方便做界面。
  let live = $state<LiveStatus | null>(null);
  // 延迟由浏览器自己测（每秒三次请求的空回答）；还没测出来时用示例数据。
  let measured = $state<ConnectionStats | null>(null);
  onMount(() => {
    const stops: (() => void)[] = [];
    // 接上真实语音时延迟已经有人测了，不用再发一遍。
    if (!stats) stops.push(watchConnection((stats) => (measured = stats)));
    stops.push(watchStatus((status) => (live = status)));
    return () => stops.forEach((stop) => stop());
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

  // 中间显示哪个文字频道：先看选中的，还没选就用第一个。
  const shownChannel = $derived(chatChannel ?? chat?.channels[0]?.id ?? null);

  // 正在当前频道里打字的人（已经去掉我自己、过期的）。
  const typingHere = $derived(
    shownChannel === null || typers === undefined
      ? []
      : typingNames(typers, shownChannel, typingNow ?? Date.now(), meName),
  );

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
  <Header
    server={server}
    me={snapshot.me}
    info={connectionInfo ?? null}
    {active}
    roomCount={rooms.length}
    {unread}
    onSelect={(key) => (active = key)}
  />

  <ChannelList
    channels={textChannels.map((channel) => ({ id: channel.id, name: channel.name }))}
    unread={unreadOf}
    selected={shownChannel}
    onSelect={(id) => {
      openThread = null;
      chatChannel = id;
      active = "chat";
    }}
    {rooms}
    {groups}
    {joinedId}
    {meName}
    onJoin={join}
    {directs}
    canCreate={canCreate ?? true}
    onCreate={createRoom}
    onCreateIn={onCreateIn}
    {roomActions}
  />

  <main class="main">
    {#if active === "chat"}
      {#if chat && chat.available}
        <ChatView
          text={chat}
          onSend={(id, body, extra) => onSendText?.(id, body, extra)}
          onUpload={onUploadText}
          onOpen={onOpenChat}
        onRead={onReadText}
          onUnread={(total, counts) => {
            unread = total;
            unreadOf = counts;
          }}
          onReact={onReactText}
          onEdit={onEditText}
          onPin={onPinText}
          myName={meName}
          {myId}
          typing={typingHere}
          onTyping={shownChannel === null ? undefined : () => onTypingText?.(shownChannel)}
          {previewOf}
          {httpBase}
          extraChannels={directs}
          names={people.map((person) => person.name)}
          {blocked}
          focusChannel={chatChannel}
          tabs={false}
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
          onThread={(id) => (threadAsk = id)}
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

  {#if threadAsk !== null}
    <Ask
      title="新话题"
      label="话题从你选的那条消息开始，之后的消息都在话题里。"
      placeholder="话题名字"
      ok="创建"
      onDone={(name) => {
        if (threadAsk !== null) onCreateThread?.(threadAsk, name);
        threadAsk = null;
      }}
      onCancel={() => (threadAsk = null)}
    />
  {/if}

  <StatusRail
    connection={connection}
    server={server}
    activity={snapshot.activity}
    {predicted}
  />

  <Drawer open={sharing} title="共享屏幕" wide onClose={() => (sharing = false)}>
    {#if screenShare}
      <div class="share">
        <iframe
          title="屏幕共享"
          src={screenShare}
          allow="camera; microphone; display-capture; fullscreen; autoplay"
        ></iframe>
        <p class="hint">在共享页里点开始共享，屏幕就会发出去。这个页面由你们自己的服务器提供，不经过我们。</p>
      </div>
    {:else}
      <div class="share">
        <p>这个服务器还没有接共享页。</p>
        <p class="hint">
          自建一个开源的会议服务（Jitsi Meet 或 Galène），把地址填进服务器的 <code>SCREEN_SHARE_URL</code>，这里就会出现它的画面。
        </p>
      </div>
    {/if}
  </Drawer>

  <VoiceBar
    roomName={joined?.name ?? null}
    count={joined?.members.length ?? 0}
    rtt={connection.rtt}
    {micOn}
    {soundOn}
    onToggleMic={toggleMic}
    onToggleSound={toggleSound}
    onLeave={leave}
    onShare={joined === null ? undefined : () => (sharing = true)}
  />
</div>

<style>
  /* 共享页：占满抽屉，下面的说明小一号，读得出来但不抢眼。 */
  .share {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .share iframe {
    width: 100%;
    height: min(62vh, 520px);
    border: 1px solid var(--sg-line);
    border-radius: 10px;
    background: #0d0f11;
  }

  .share p {
    margin: 0;
    font-size: 13px;
  }

  .share .hint {
    color: var(--sg-muted);
    font-size: 12.5px;
    line-height: 1.6;
  }

  .share code {
    padding: 1px 5px;
    border-radius: 4px;
    background: var(--sg-panel);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px;
  }
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

</style>
