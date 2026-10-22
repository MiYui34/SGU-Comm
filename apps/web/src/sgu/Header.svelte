<script lang="ts">
  import type { Me, NavKey, ServerInfo } from "./types";

  import ConnectionInfo from "../components/ConnectionInfo.svelte";
  import type { ConnectionInfo as Info } from "../lib/connectionInfo.svelte";

  // 导航放在顶栏中间：左边是房间和聊天，右边是状态。
  let {
    server,
    me,
    info,
    active,
    roomCount,
    unread,
    onSelect,
  }: {
    server: ServerInfo;
    me: Me;
    info?: Info | null;
    active: NavKey;
    roomCount: number;
    unread: number;
    onSelect: (key: NavKey) => void;
  } = $props();

  const items: { key: NavKey; label: string }[] = [
    { key: "chat", label: "聊天" },
    { key: "voice", label: "语音" },
    { key: "players", label: "玩家" },
    { key: "settings", label: "设置" },
  ];
</script>

<header class="head">
  <div class="mark">SGU</div>
  <div class="brand">
    SGU 生存服
    {#if server.version}<span class="ver">{server.version}</span>{/if}
  </div>

  <nav class="nav">
    {#each items as item (item.key)}
      <button
        type="button"
        class="tab"
        class:on={active === item.key}
        onclick={() => onSelect(item.key)}
      >
        {item.label}
        {#if item.key === "voice" && roomCount > 0}
          <span class="count">{roomCount}</span>
        {:else if item.key === "chat" && unread > 0}
          <span class="count new">{unread}</span>
        {/if}
      </button>
    {/each}
  </nav>

  <div class="right">
    {#if server.slots > 0}
      <div class="online"><span class="dot"></span><b>{server.online}</b> / {server.slots} 在线</div>
    {:else}
      <div class="off">游戏服未连接</div>
    {/if}
    {#if info}
      <ConnectionInfo info={info} />
    {/if}
    <div class="me">
      <span class="av">{me.initial}</span>
      <span class="name">{me.name}</span>
    </div>
  </div>
</header>

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 14px;
    grid-area: head;
    height: 60px;
    padding: 0 18px;
    background: var(--sg-panel);
    border-bottom: 1px solid var(--sg-line);
  }

  .mark {
    display: grid;
    flex: none;
    place-items: center;
    width: 32px;
    height: 32px;
    color: #fff;
    background: var(--sg-green);
    border-radius: 8px;
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.5px;
  }

  .brand {
    display: flex;
    flex: none;
    align-items: baseline;
    gap: 8px;
    font-size: 15px;
    font-weight: 600;
  }

  .ver {
    color: var(--sg-faint);
    font-size: 11.5px;
    font-weight: 500;
  }

  .nav {
    display: flex;
    gap: 2px;
    margin-left: 10px;
    padding: 3px;
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 9px;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 13px;
    color: var(--sg-muted);
    font-size: 13px;
    border-radius: 7px;
  }

  .tab:hover {
    color: var(--sg-text);
  }

  .tab.on {
    color: var(--sg-text);
    background: var(--sg-card);
    box-shadow: 0 1px 2px rgb(44 38 29 / 7%);
    font-weight: 600;
  }

  .count {
    color: var(--sg-faint);
    font-size: 11px;
    font-weight: 500;
  }

  .count.new {
    min-width: 17px;
    padding: 0 5px;
    color: #fdfbf6;
    background: var(--sg-green);
    border-radius: 99px;
    font-weight: 600;
    line-height: 16px;
    text-align: center;
  }

  .right {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-left: auto;
  }

  .online {
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--sg-muted);
    font-size: 13px;
  }

  .online b {
    color: var(--sg-text);
    font-weight: 600;
  }

  .dot {
    flex: none;
    width: 7px;
    height: 7px;
    background: var(--sg-green);
    border-radius: 50%;
  }

  .off {
    color: var(--sg-muted);
    font-size: 13px;
    opacity: 0.75;
  }

  .me {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .av {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    color: #fff;
    background: #4a6fa5;
    border-radius: 7px;
    font-size: 11px;
    font-weight: 600;
  }

  .name {
    color: var(--sg-muted);
    font-size: 13px;
  }
</style>
