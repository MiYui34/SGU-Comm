<script lang="ts">
  import Icon from "./Icon.svelte";
  import ContextMenu from "./ContextMenu.svelte";
  import ConnectionInfo from "./ConnectionInfo.svelte";
  import type { ConnectionInfo as Info } from "../lib/connectionInfo.svelte";
  import type { GoLiveAction, GoLiveEntry } from "../lib/stream/goLive";

  // Above the user panel while in a voice channel, as in Discord: where you
  // are, going live, and the button to leave.
  // entries: the "Go live" menu (see goLiveEntries; none: no button), live:
  // one of our streams runs. music: music mode is on (see
  // Settings.musicMode). info: the connection's quality (see ConnectionInfo).
  let {
    channel,
    onLeave,
    music = false,
    recording = false,
    live = false,
    entries = [],
    onGoLive,
    info,
  }: {
    channel: string;
    onLeave: () => void;
    music?: boolean;
    // The channel is being recorded.
    recording?: boolean;
    live?: boolean;
    entries?: GoLiveEntry[];
    onGoLive?: (action: GoLiveAction) => void;
    info: Info;
  } = $props();

  let menu = $state<{ x: number; y: number } | null>(null);
  const choose = (action: GoLiveAction) => {
    menu = null;
    onGoLive?.(action);
  };
</script>

<section class="bar" aria-label="Voice">
  <div class="where">
    <div class="state">
      <ConnectionInfo {info} />
      Voice connected
      {#if music}<span class="note" title="No filters, better sound (see the settings)">Music mode</span>{/if}
      {#if recording}<span class="note rec" title="Everything said in this channel is recorded">● Recording</span>{/if}
    </div>
    <div class="channel ellipsis" title={channel}>{channel}</div>
  </div>
  {#if entries.length && onGoLive}
    <button
      class="live"
      class:on={live}
      title="Go live"
      aria-label="Go live"
      aria-pressed={live}
      aria-haspopup="menu"
      aria-expanded={!!menu}
      onclick={(e) => {
        const r = e.currentTarget.getBoundingClientRect();
        menu = { x: r.left, y: r.top - 4 };
      }}
    >
      <Icon name="screen" size={20} />
    </button>
  {/if}
  <button title="Leave voice" aria-label="Leave voice" onclick={onLeave}>
    <Icon name="hangup" size={20} />
  </button>
</section>

{#if menu}
  <ContextMenu x={menu.x} y={menu.y} up label="Go live" onClose={() => (menu = null)}>
    {#each entries as e (e.action)}
      <button class="item" disabled={!!e.off} onclick={() => choose(e.action)}>
        {e.label}
        {#if e.off}<span class="why">{e.off}</span>{/if}
      </button>
    {/each}
  </ContextMenu>
{/if}

<style>
  .bar {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 8px 8px 0;
    padding: 8px 8px 8px 12px;
    border-radius: 8px;
    background: var(--bg-input);
  }
  .where {
    flex: 1;
    min-width: 0;
  }
  .state {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 2px;
    color: var(--accent);
    font-size: 13px;
    font-weight: 600;
  }
  .note {
    margin-left: 4px;
    padding: 0 4px;
    border-radius: 4px;
    background: var(--bg-hover);
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 500;
  }
  .rec {
    background: var(--danger);
    color: white;
  }
  .channel {
    color: var(--text-muted);
    font-size: 12px;
  }
  .bar button {
    display: inline-flex;
    padding: 6px;
    border-radius: 6px;
    color: var(--text-muted);
  }
  .bar button:hover {
    background: var(--bg-hover);
    color: var(--danger);
  }
  .bar .live:hover {
    color: var(--text);
  }
  .bar .live.on {
    color: var(--accent);
  }
  .item {
    display: block;
    width: 100%;
    padding: 8px;
    border-radius: 4px;
    font-size: 14px;
    text-align: left;
    color: var(--text-muted);
  }
  .item:not(:disabled):hover {
    background: var(--accent-strong);
    color: white;
  }
  .why {
    display: block;
    font-size: 12px;
    color: var(--text-faint);
  }
</style>
