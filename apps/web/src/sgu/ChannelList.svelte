<script lang="ts">
  import { tick } from "svelte";
  import { initialOf, playerColor, type RoomAction, type VoiceRoom } from "./types";

  // 左栏：文字频道在上，语音房间在下。点文字频道换中间的消息，
  // 点语音房间就是走进去说话。
  let {
    channels,
    unread,
    selected,
    onSelect,
    directs = [],
    rooms,
    joinedId,
    meName,
    onJoin,
    canCreate = true,
    onCreate,
    roomActions,
  }: {
    channels: { id: number; name: string }[];
    // 每个频道还有几条没看；没有就不显示。
    unread: Record<number, number>;
    selected: number | null;
    onSelect: (channelId: number) => void;
    // 私信：和某个人的一对一聊天，和文字频道一样点开。
    directs?: { id: number; name: string }[];
    rooms: VoiceRoom[];
    joinedId: string | null;
    meName: string;
    onJoin: (roomId: string) => void;
    canCreate?: boolean;
    onCreate?: (name: string) => void;
    roomActions?: (roomId: string) => RoomAction[];
  } = $props();

  let menuFor = $state<string | null>(null);
  let creating = $state(false);
  let draft = $state("");
  let field = $state<HTMLInputElement | null>(null);
  // 人多的房间默认只列前二十个，展开过的记在这里。
  let opened = $state<Record<string, boolean>>({});

  async function startCreating() {
    creating = true;
    draft = "";
    menuFor = null;
    await tick();
    field?.focus();
  }

  function submit(event: SubmitEvent) {
    event.preventDefault();
    const name = draft.trim();
    if (name === "") return;
    onCreate?.(name);
    creating = false;
    draft = "";
  }

  const membersIn = (room: VoiceRoom) =>
    room.members.some((name) => name === meName)
      ? `你${room.members.length > 1 ? `、${room.members.length - 1} 人` : ""}`
      : room.members.length === 0
        ? ""
        : `${room.members.length} 人`;

  // 一个房间最多先列出这么多人，多出来的折起来。
  const foldAfter = 20;

  // 双击才进语音；键盘的回车/空格算数（那种点击 detail 是 0）。
  function pick(event: MouseEvent, room: VoiceRoom) {
    if (event.detail === 0) onJoin(room.id);
  }

  const shownIn = (room: VoiceRoom) =>
    opened[room.id] === true ? room.members : room.members.slice(0, foldAfter);

  function fold(room: VoiceRoom) {
    opened = { ...opened, [room.id]: opened[room.id] !== true };
  }
</script>

<aside class="side">
  <div class="group">
    <div class="group-head">
      <span>文字频道</span>
    </div>

    {#if channels.length === 0}
      <p class="none">还没有频道</p>
    {/if}

    {#each channels as channel (channel.id)}
      <button
        type="button"
        class="row text"
        class:on={channel.id === selected}
        onclick={() => onSelect(channel.id)}
      >
        <span class="hash">#</span>
        <span class="label">{channel.name}</span>
        {#if (unread[channel.id] ?? 0) > 0}
          <span class="dot">{unread[channel.id]}</span>
        {/if}
      </button>
    {/each}
  </div>

  {#if directs.length > 0}
    <div class="group">
      <div class="group-head">
        <span>私信</span>
      </div>
      {#each directs as peer (peer.id)}
        <button
          type="button"
          class="row text"
          class:on={peer.id === selected}
          onclick={() => onSelect(peer.id)}
        >
          <span class="hash">@</span>
          <span class="label">{peer.name}</span>
          {#if (unread[peer.id] ?? 0) > 0}
            <span class="dot">{unread[peer.id]}</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}

  <div class="group">
    <div class="group-head">
      <span>语音房间</span>
      {#if canCreate}
        <button type="button" class="add" aria-label="创建房间" onclick={startCreating}>＋</button>
      {/if}
    </div>

    {#each rooms as room (room.id)}
      <div class="slot">
        <button
          type="button"
          class="row voice"
          class:on={room.id === joinedId}
          title={room.id === joinedId ? "你在这个房间" : "双击加入语音"}
          onclick={(event) => pick(event, room)}
          ondblclick={() => onJoin(room.id)}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M11 5 6 9H3v6h3l5 4V5Z" />
            <path d="M15.5 8.5a5 5 0 0 1 0 7" />
          </svg>
          <span class="label">{room.name}</span>
          <span class="count">{membersIn(room)}</span>
        </button>

        {#if (roomActions?.(room.id) ?? []).length > 0}
          <button
            type="button"
            class="more"
            aria-label="管理这个房间"
            onclick={() => (menuFor = menuFor === room.id ? null : room.id)}
          >
            ⋯
          </button>
          {#if menuFor === room.id}
            <div class="menu">
              {#each roomActions?.(room.id) ?? [] as action (action.key)}
                <button
                  type="button"
                  onclick={() => {
                    menuFor = null;
                    action.run();
                  }}
                >
                  {action.label}
                </button>
              {/each}
            </div>
          {/if}
        {/if}

        {#if room.members.length > 0}
          <div class="people">
            {#each shownIn(room) as name (name)}
              <span class="person">
                <span class="face" style:background={playerColor(name)}>{initialOf(name)}</span>
                <span class="name">{name}</span>
              </span>
            {/each}
            {#if room.members.length > foldAfter}
              <button type="button" class="fold" onclick={() => fold(room)}>
                {opened[room.id] === true ? "收起" : `还有 ${room.members.length - foldAfter} 人`}
              </button>
            {/if}
          </div>
        {/if}
      </div>
    {/each}

    {#if creating}
      <form class="make" onsubmit={submit}>
        <input
          bind:this={field}
          bind:value={draft}
          placeholder="房间名字"
          maxlength="24"
          aria-label="房间名字"
        />
        <div class="make-acts">
          <button type="submit" class="ok" disabled={draft.trim() === ""}>创建</button>
          <button type="button" class="no" onclick={() => (creating = false)}>取消</button>
        </div>
      </form>
    {/if}
  </div>
</aside>

<style>
  .side {
    display: flex;
    flex-direction: column;
    gap: 18px;
    grid-area: side;
    min-height: 0;
    overflow-y: auto;
    padding: 14px 10px 16px;
    background: var(--sg-panel);
    border-right: 1px solid var(--sg-line);
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .group-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 8px 6px;
    color: var(--sg-faint);
    font-size: 11.5px;
    letter-spacing: 0.4px;
  }

  .add {
    width: 20px;
    height: 20px;
    padding: 0;
    color: var(--sg-faint);
    font-size: 15px;
    line-height: 1;
    border-radius: 6px;
  }

  .add:hover {
    background: rgb(44 38 29 / 7%);
    color: var(--sg-text);
  }

  .row {
    display: flex;
    align-items: center;
    gap: 7px;
    width: 100%;
    padding: 7px 9px;
    color: var(--sg-muted);
    font-size: 13px;
    text-align: left;
    border-radius: 7px;
  }

  .row:hover {
    background: rgb(44 38 29 / 5%);
    color: var(--sg-text);
  }

  .row.on {
    color: var(--sg-text);
    background: var(--sg-card);
    box-shadow: 0 1px 2px rgb(44 38 29 / 6%);
    font-weight: 600;
  }

  .hash {
    flex: none;
    width: 13px;
    color: var(--sg-faint);
    font-weight: 500;
    text-align: center;
  }

  .row.on .hash {
    color: var(--sg-green);
  }

  .row svg {
    flex: none;
    width: 15px;
    height: 15px;
    opacity: 0.65;
  }

  .row.on svg {
    color: var(--sg-green);
    opacity: 1;
  }

  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .count {
    flex: none;
    margin-left: auto;
    color: var(--sg-faint);
    font-size: 11.5px;
    font-weight: 500;
  }

  .row.on .count {
    color: var(--sg-green);
  }

  .dot {
    flex: none;
    min-width: 17px;
    margin-left: auto;
    padding: 0 5px;
    color: #fdfbf6;
    background: var(--sg-green);
    border-radius: 99px;
    font-size: 10.5px;
    font-weight: 600;
    line-height: 16px;
    text-align: center;
  }

  /* 房间行右边要空出「⋯」的位置，不然人多的房间会压到按钮上。 */
  .slot {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .slot .row {
    padding-right: 30px;
  }

  /* 房间里有谁：头像加名字，排不下就换行，名字长的截断。 */
  .people {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 10px;
    padding: 1px 9px 5px 31px;
  }

  .person {
    display: inline-flex;
    gap: 5px;
    align-items: center;
    min-width: 0;
    max-width: 100%;
  }

  .face {
    display: inline-flex;
    flex: none;
    align-items: center;
    justify-content: center;
    width: 17px;
    height: 17px;
    color: #fdfbf6;
    font-size: 9.5px;
    font-weight: 600;
    line-height: 1;
    border-radius: 50%;
  }

  .name {
    overflow: hidden;
    color: var(--sg-muted);
    font-size: 11.5px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .fold {
    padding: 0;
    color: var(--sg-green);
    font-size: 11.5px;
  }

  .fold:hover {
    text-decoration: underline;
  }

  .more {
    position: absolute;
    top: 3px;
    right: 5px;
    padding: 0 5px;
    color: var(--sg-muted);
    font-size: 13px;
    line-height: 18px;
    border-radius: 5px;
    opacity: 0;
  }

  .slot:hover .more,
  .more:focus-visible {
    opacity: 1;
  }

  .more:hover {
    background: rgb(44 38 29 / 9%);
  }

  .menu {
    position: absolute;
    top: 26px;
    right: 4px;
    z-index: 7;
    display: flex;
    flex-direction: column;
    min-width: 118px;
    padding: 4px;
    background: var(--sg-card);
    border: 1px solid var(--sg-line);
    border-radius: 9px;
    box-shadow: 0 12px 28px rgb(44 38 29 / 14%);
  }

  .menu button {
    padding: 6px 10px;
    color: var(--sg-text);
    font-size: 12.5px;
    text-align: left;
    border-radius: 6px;
  }

  .menu button:hover {
    background: var(--sg-panel);
  }

  .none {
    margin: 0;
    padding: 2px 9px 6px;
    color: var(--sg-faint);
    font-size: 12px;
  }

  .make {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 6px 8px 2px;
  }

  .make input {
    width: 100%;
    padding: 6px 8px;
    font: inherit;
    font-size: 12.5px;
    color: var(--sg-text);
    background: var(--sg-card);
    border: 1px solid var(--sg-line-hi);
    border-radius: 7px;
  }

  .make input:focus {
    border-color: var(--sg-green);
    outline: none;
  }

  .make-acts {
    display: flex;
    gap: 6px;
  }

  .make-acts button {
    padding: 4px 10px;
    font-size: 12px;
    border-radius: 6px;
  }

  .make-acts .ok {
    color: #fdfbf6;
    background: var(--sg-green);
  }

  .make-acts .ok:disabled {
    color: var(--sg-faint);
    background: var(--sg-panel);
  }

  .make-acts .no {
    color: var(--sg-muted);
    border: 1px solid var(--sg-line-hi);
  }
</style>
