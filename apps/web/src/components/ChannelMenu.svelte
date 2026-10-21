<script lang="ts">
  import ContextMenu from "./ContextMenu.svelte";
  import { isMuted, levelOf, LEVELS, MUTE_FOR, mutedUntilText, muteUntil, type ChannelPref } from "../lib/channelPrefs";

  // Right-click menu of a channel, or of a direct chat (direct). Items show
  // only when the user may use them (the server checks again).
  let {
    name,
    x,
    y,
    canEdit = false,
    canDelete = false,
    canMoveUp = false,
    canMoveDown = false,
    canMembers = false,
    canModerators = false,
    canCreateSub = false,
    recording,
    onRecord,
    notify,
    onNotify,
    onEdit,
    onCreateSub,
    onMove,
    onDelete,
    onMembers,
    onModerators,
    direct,
    thread,
    category = false,
    onClose,
  }: {
    name: string;
    x: number;
    y: number;
    canEdit?: boolean;
    canDelete?: boolean;
    canMoveUp?: boolean;
    canMoveDown?: boolean;
    // A private channel's members (staff and its moderators); the
    // channel's moderators (staff).
    canMembers?: boolean;
    canModerators?: boolean;
    // Voice channels: a subchannel in it (MakeChannel there).
    canCreateSub?: boolean;
    // Voice channels (staff): being recorded or not; undefined: may not
    // record.
    recording?: boolean;
    onRecord?: (on: boolean) => void;
    // Text channels and direct chats: notification settings (see
    // lib/channelPrefs; direct chats only mute); undefined: none. desktop:
    // desktop notifications are on.
    notify?: { pref: ChannelPref; desktop: boolean };
    onNotify?: (patch: ChannelPref) => void;
    onEdit?: () => void;
    onCreateSub?: () => void;
    onMove?: (step: -1 | 1) => void;
    onDelete?: () => void;
    onMembers?: () => void;
    onModerators?: () => void;
    // A direct chat (see lib/directList).
    direct?: { pinned: boolean; onProfile: () => void; onRead?: () => void; onPin: () => void; onHide: () => void };
    // A thread (see lib/threads): follow it, archive it (or open it again;
    // undefined: may not), take its channel's notification settings again
    // (undefined: it does).
    thread?: { following: boolean; onFollow: () => void; archived: boolean; onArchive?: () => void; onInherit?: () => void };
    // A text category (see lib/categories).
    category?: boolean;
    onClose: () => void;
  } = $props();

  const act = (fn: () => void) => () => {
    fn();
    onClose();
  };
  const id = $props.id();
  const noun = $derived(direct ? "conversation" : thread ? "thread" : category ? "category" : "channel");
  const changes = $derived(canEdit || canCreateSub || canMoveUp || canMoveDown || canMembers || canModerators);
</script>

<ContextMenu {x} {y} label="Actions for {name}" {onClose}>
  {#if thread}
    <button onclick={act(thread.onFollow)}>{thread.following ? "Unfollow thread" : "Follow thread"}</button>
    {#if thread.onArchive}<button onclick={act(thread.onArchive)}>{thread.archived ? "Open thread again" : "Archive thread"}</button>{/if}
    <div class="sep"></div>
  {/if}
  {#if direct}
    <button onclick={act(direct.onProfile)}>Profile</button>
    {#if direct.onRead}<button onclick={act(direct.onRead)}>Mark as read</button>{/if}
    <div class="sep"></div>
  {/if}
  {#if notify && onNotify}
    {#if isMuted(notify.pref, Date.now())}
      {@const end = mutedUntilText(notify.pref.mutedUntil)}
      <button onclick={act(() => onNotify({ mutedUntil: undefined }))}>Unmute {noun}</button>
      {#if end}<p class="none">Muted until {end}</p>{/if}
    {:else}
      <div role="group" aria-labelledby="{id}-mute">
        <p class="heading" id="{id}-mute">Mute {noun}</p>
        {#each MUTE_FOR as [label, minutes] (minutes)}
          <button onclick={act(() => onNotify({ mutedUntil: muteUntil(minutes, Date.now()) }))}>{label}</button>
        {/each}
      </div>
    {/if}
  {/if}
  {#if direct}
    <div class="sep"></div>
    <button onclick={act(direct.onPin)}>{direct.pinned ? "Unpin" : "Pin to top"}</button>
    <button onclick={act(direct.onHide)}>Close conversation</button>
  {:else if notify && onNotify}
    <div class="sep"></div>
    <div role="group" aria-labelledby="{id}-level">
      <p class="heading" id="{id}-level">Notifications</p>
      {#each LEVELS as [level, label] (level)}
        <button class="choice" aria-pressed={levelOf(notify.pref) === level} onclick={act(() => onNotify({ level }))}>{label}</button>
      {/each}
      {#if thread?.onInherit}<button onclick={act(thread.onInherit)}>Same as the channel</button>{/if}
      {#if !notify.desktop}<p class="none">Desktop notifications are off (Settings).</p>{/if}
    </div>
    {#if changes}<div class="sep"></div>{/if}
  {/if}
  {#if recording !== undefined && onRecord}
    {#if recording}
      <button class="danger" onclick={act(() => onRecord(false))}>Stop recording</button>
    {:else}
      <button onclick={act(() => onRecord(true))}>Record channel</button>
    {/if}
    {#if changes}<div class="sep"></div>{/if}
  {/if}
  {#if changes}
    {#if canEdit && onEdit}<button onclick={act(onEdit)}>Edit {noun}</button>{/if}
    {#if canCreateSub && onCreateSub}<button onclick={act(onCreateSub)}>Create subchannel</button>{/if}
    {#if canMembers && onMembers}<button onclick={act(onMembers)}>Members…</button>{/if}
    {#if canModerators && onModerators}<button onclick={act(onModerators)}>Moderators…</button>{/if}
    {#if canMoveUp}<button onclick={act(() => onMove?.(-1))}>Move up</button>{/if}
    {#if canMoveDown}<button onclick={act(() => onMove?.(1))}>Move down</button>{/if}
    {#if canDelete && onDelete}
      <div class="sep"></div>
      <button class="danger" onclick={act(onDelete)}>Delete {noun}</button>
    {/if}
  {:else if !notify && recording === undefined && !direct && !thread}
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
