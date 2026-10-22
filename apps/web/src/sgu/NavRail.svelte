<script lang="ts">
  import type { NavKey } from "./types";

  let {
    active,
    roomCount,
    unread,
    onSelect,
  }: {
    active: NavKey;
    roomCount: number;
    unread: number;
    onSelect: (key: NavKey) => void;
  } = $props();

  const items: { key: NavKey; label: string }[] = [
    { key: "voice", label: "语音房间" },
    { key: "chat", label: "聊天" },
    { key: "players", label: "玩家" },
    { key: "settings", label: "设置" },
  ];
</script>

<nav class="nav">
  {#each items as item (item.key)}
    <button
      type="button"
      class="item"
      class:on={active === item.key}
      onclick={() => onSelect(item.key)}
    >
      {#if item.key === "voice"}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <path d="M11 5 6 9H3v6h3l5 4V5Z" />
          <path d="M15.5 8.5a5 5 0 0 1 0 7" />
          <path d="M18.5 5.5a9 9 0 0 1 0 13" />
        </svg>
      {:else if item.key === "chat"}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <path d="M7 4h10a3 3 0 0 1 3 3v6a3 3 0 0 1-3 3h-5l-4 3.2V16H7a3 3 0 0 1-3-3V7a3 3 0 0 1 3-3Z" />
          <path d="M7.5 8.5h9" />
          <path d="M7.5 12h5.5" />
        </svg>
      {:else if item.key === "players"}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
          <circle cx="9" cy="7" r="4" />
          <path d="M22 21v-2a4 4 0 0 0-3-3.9" />
        </svg>
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-1.8-.3 1.6 1.6 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1A1.6 1.6 0 0 0 9 19.4a1.6 1.6 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.6 1.6 0 0 0 .3-1.8 1.6 1.6 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1A1.6 1.6 0 0 0 4.6 9a1.6 1.6 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 1.8.3H9a1.6 1.6 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.6 1.6 0 0 0 1 1.5 1.6 1.6 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0-.3 1.8V9a1.6 1.6 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.6 1.6 0 0 0-1.5 1Z" />
        </svg>
      {/if}
      {item.label}
      {#if item.key === "voice"}
        <span class="badge">{roomCount}</span>
      {:else if item.key === "chat" && unread > 0}
        <span class="badge new">{unread}</span>
      {/if}
    </button>
  {/each}
</nav>

<style>
  .nav {
    grid-area: nav;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-height: 0;
    padding: 16px 12px;
    border-right: 1px solid var(--sg-line);
    background: var(--sg-panel);
  }

  .item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 9px 11px;
    border-radius: 8px;
    color: var(--sg-muted);
    font-size: 13.5px;
    text-align: left;
  }

  .item svg {
    flex: none;
    width: 17px;
    height: 17px;
    opacity: 0.7;
  }

  .item:hover {
    background: rgb(44 38 29 / 5%);
    color: var(--sg-text);
  }

  .item.on {
    background: var(--sg-card);
    box-shadow: 0 1px 2px rgb(44 38 29 / 6%);
    color: var(--sg-text);
    font-weight: 600;
  }

  .item.on svg {
    color: var(--sg-green);
    opacity: 1;
  }

  .badge {
    margin-left: auto;
    color: var(--sg-faint);
    font-size: 11px;
  }

  .badge.new {
    color: var(--sg-green);
    font-weight: 600;
  }
</style>
