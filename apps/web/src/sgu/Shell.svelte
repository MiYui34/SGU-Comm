<script lang="ts">
  import Header from "./Header.svelte";
  import NavRail from "./NavRail.svelte";
  import PlayersView from "./PlayersView.svelte";
  import RoomGrid from "./RoomGrid.svelte";
  import SettingsView from "./SettingsView.svelte";
  import StatusRail from "./StatusRail.svelte";
  import VoiceBar from "./VoiceBar.svelte";
  import ChatView from "./ChatView.svelte";
  import { onMount, untrack } from "svelte";
  import { watchConnection, watchStatus, type LiveStatus } from "./live";
  import { joinRoom, leaveRooms } from "./rooms";
  import type { TextState } from "../lib/textChannels";
  import {
    playerOfName,
    type ConnectionStats,
    type NavKey,
    type Person,
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
    myId = null,
  }: {
    snapshot: SguState;
    // 接上真实语音时由外面给房间和当前房间；不给就自己管示例数据。
    rooms?: VoiceRoom[];
    joinedRoomId?: string | null;
    stats?: ConnectionStats | null;
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
  } = $props();

  let active = $state<NavKey>("voice");
  // 下面这几处只取挂载时的初值；之后的连接状态由 Shell 自己维护。
  let localJoined = $state<string | null>(untrack(() => snapshot.joinedRoomId));
  let localMic = $state(true);
  let localSound = $state(true);
  let unread = $state(0);

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
  <Header server={server} me={snapshot.me} />

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
        />
      {:else if chat}
        <p class="todo">这个服务器没有开文字频道</p>
      {:else}
        <p class="todo">聊天还没接上</p>
      {/if}
    {:else if active === "players"}
      <PlayersView
        {people}
        game={players}
        {meName}
        {joinedId}
        onJoin={onJoin ? join : undefined}
      />
    {:else if active === "settings"}
      <SettingsView {meName} {micOn} {soundOn} onToggleMic={toggleMic} onToggleSound={toggleSound} />
    {:else}
      <div class="head-row">
        <h2>语音房间</h2>
        <span>{rooms.length} 个</span>
      </div>

      <RoomGrid {rooms} {joinedId} {meName} onJoin={join} onCreate={createRoom} canCreate={canCreate ?? true} />
    {/if}
  </main>

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
</style>
