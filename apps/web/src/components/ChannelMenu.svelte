<script lang="ts">
  import ContextMenu from "./ContextMenu.svelte";
  import { isMuted, levelOf, LEVELS, type ChannelPref } from "../lib/channelPrefs";

  // Right-click menu of a channel. Items show only when the user may use
  // them (the server checks again).
  let {
    name,
    x,
    y,
    canEdit,
    canDelete,
    canMoveUp,
    canMoveDown,
    canMembers = false,
    notify,
    onNotify,
    onEdit,
    onMove,
    onDelete,
    onMembers,
    onClose,
  }: {
    name: string;
    x: number;
    y: number;
    canEdit: boolean;
    canDelete: boolean;
    canMoveUp: boolean;
    canMoveDown: boolean;
    // A private channel's members (admins and moderators).
    canMembers?: boolean;
    // Text channels: notification settings (see lib/channelPrefs);
    // undefined: none. desktop: desktop notifications are on.
    notify?: { pref: ChannelPref; desktop: boolean };
    onNotify?: (patch: ChannelPref) => void;
    onEdit: () => void;
    onMove: (step: -1 | 1) => void;
    onDelete: () => void;
    onMembers?: () => void;
    onClose: () => void;
  } = $props();

  const act = (fn: () => void) => () => {
    fn();
    onClose();
  };
  const MUTE_FOR: [string, number][] = [
    ["For 15 minutes", 15],
    ["For 1 hour", 60],
    ["For 8 hours", 8 * 60],
    ["For 24 hours", 24 * 60],
    ["Until I turn it back on", 0],
  ];
  const id = $props.id();
  const until = (t: number | undefined) => (t ? new Date(t).toLocaleString(undefined, { weekday: "short", hour: "numeric", minute: "2-digit" }) : null);
</script>

<ContextMenu {x} {y} label="Actions for {name}" {onClose}>
  {#if notify && onNotify}
    {#if isMuted(notify.pref, Date.now())}
      {@const end = until(notify.pref.mutedUntil)}
      <button onclick={act(() => onNotify({ mutedUntil: undefined }))}>Unmute channel</button>
      {#if end}<p class="none">Muted until {end}</p>{/if}
    {:else}
      <div role="group" aria-labelledby="{id}-mute">
        <p class="heading" id="{id}-mute">Mute channel</p>
        {#each MUTE_FOR as [label, minutes] (minutes)}
          <button onclick={act(() => onNotify({ mutedUntil: minutes && Date.now() + minutes * 60_000 }))}>{label}</button>
        {/each}
      </div>
    {/if}
    <div class="sep"></div>
    <div role="group" aria-labelledby="{id}-level">
      <p class="heading" id="{id}-level">Notifications</p>
      {#each LEVELS as [level, label] (level)}
        <button class="choice" aria-pressed={levelOf(notify.pref) === level} onclick={act(() => onNotify({ level }))}>{label}</button>
      {/each}
      {#if !notify.desktop}<p class="none">Desktop notifications are off (Settings).</p>{/if}
    </div>
    {#if canEdit || canMoveUp || canMoveDown}<div class="sep"></div>{/if}
  {/if}
  {#if canEdit || canMoveUp || canMoveDown}
    {#if canEdit}<button onclick={act(onEdit)}>Edit channel</button>{/if}
    {#if canMembers && onMembers}<button onclick={act(onMembers)}>Members…</button>{/if}
    {#if canMoveUp}<button onclick={act(() => onMove(-1))}>Move up</button>{/if}
    {#if canMoveDown}<button onclick={act(() => onMove(1))}>Move down</button>{/if}
    {#if canDelete}
      <div class="sep"></div>
      <button class="danger" onclick={act(onDelete)}>Delete channel</button>
    {/if}
  {:else if !notify}
    <p class="none">You may not change this channel.</p>
  {/if}
</ContextMenu>

<style>
  .sep {
    height: 1px;
    margin: 4px 6px;
    background: var(--border-subtle);
  }
  button {
    display: block;
    width: 100%;
    padding: 8px;
    border-radius: 4px;
    font-size: 14px;
    text-align: left;
    color: var(--text-muted);
  }
  button:hover {
    background: var(--accent-strong);
    color: white;
  }
  .danger {
    color: var(--danger);
  }
  .danger:hover {
    background: var(--danger);
  }
  .heading {
    margin: 0;
    padding: 4px 8px;
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .choice[aria-pressed="true"]::after {
    content: "✓" / "";
    float: right;
  }
  .none {
    margin: 0;
    padding: 8px;
    font-size: 14px;
    color: var(--text-muted);
  }
</style>
