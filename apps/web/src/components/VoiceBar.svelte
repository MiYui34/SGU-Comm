<script lang="ts">
  import Icon from "./Icon.svelte";
  import ConnectionInfo from "./ConnectionInfo.svelte";
  import type { ConnectionInfo as Info } from "../lib/connectionInfo.svelte";

  // Above the user panel while in a voice channel, as in Discord: where you
  // are, and the button to leave.
  // onShare: share the screen, or stop (undefined: not possible here).
  // music: music mode is on (see Settings.musicMode). info: the
  // connection's quality (see ConnectionInfo).
  let {
    channel,
    onLeave,
    music = false,
    sharing = false,
    onShare,
    info,
  }: { channel: string; onLeave: () => void; music?: boolean; sharing?: boolean; onShare?: () => void; info: Info } = $props();
</script>

<section class="bar" aria-label="Voice">
  <div class="where">
    <div class="state">
      <ConnectionInfo {info} />
      Voice connected
      {#if music}<span class="note" title="No filters, better sound (see the settings)">Music mode</span>{/if}
    </div>
    <div class="channel ellipsis" title={channel}>{channel}</div>
  </div>
  {#if onShare}
    <button
      class="share"
      class:on={sharing}
      title={sharing ? "Stop sharing your screen" : "Share your screen"}
      aria-label={sharing ? "Stop sharing your screen" : "Share your screen"}
      aria-pressed={sharing}
      onclick={onShare}
    >
      <Icon name="screen" size={20} />
    </button>
  {/if}
  <button title="Leave voice" aria-label="Leave voice" onclick={onLeave}>
    <Icon name="hangup" size={20} />
  </button>
</section>

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
  .channel {
    color: var(--text-muted);
    font-size: 12px;
  }
  button {
    display: inline-flex;
    padding: 6px;
    border-radius: 6px;
    color: var(--text-muted);
  }
  button:hover {
    background: var(--bg-hover);
    color: var(--danger);
  }
  .share:hover {
    color: var(--text);
  }
  .share.on {
    color: var(--accent);
  }
</style>
