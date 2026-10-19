<script lang="ts">
  import { channelTree, usersIn, type ChannelNode, type ServerState } from "@kallisti/protocol";
  import { longPress } from "../lib/longPress";
  import { mirrorIds, withoutMirrors } from "../lib/mirrors";
  import { htmlToText } from "../lib/htmlText";
  import type { ChannelKind } from "../lib/channelForm";
  import type { Peer, TextChannel } from "../lib/textChannels";
  import { isCamera, streamId } from "../lib/stream/protocol";
  import Avatar from "./Avatar.svelte";
  import Badge from "./Badge.svelte";
  import Icon from "./Icon.svelte";

  let {
    server,
    currentChannel,
    lobby = false,
    speaking,
    localMuted,
    unread,
    textChannels,
    currentText,
    unreadText,
    mutedText = new Set(),
    mentionText = {},
    mentionVoice = {},
    listenOnly = {},
    privateChannels = {},
    recording = new Set(),
    directs = [],
    live = new Set(),
    previews = {},
    onWatch,
    currentDirect = null,
    onOpenDirect,
    onOpenText,
    onJoin,
    onUserMenu,
    onCreate,
    canCreate = { text: false, voice: false },
    onChannelMenu,
    onDirectMenu,
  }: {
    server: ServerState;
    currentChannel: number | null;
    // Root is a lobby without voice: only the channels below it are voice
    // channels, and users in root are not shown here.
    lobby?: boolean;
    speaking: ReadonlySet<number>;
    localMuted: ReadonlySet<number>;
    // Unread private messages per user name.
    unread: Record<string, number>;
    textChannels: TextChannel[];
    currentText: number | null;
    unreadText: ReadonlySet<number>;
    mutedText?: ReadonlySet<number>;
    // Unseen mentions of us, by text channel and by voice channel.
    mentionText?: Record<number, number>;
    mentionVoice?: Record<number, number>;
    // Listen-only voice channels (only some users may speak).
    listenOnly?: Readonly<Record<number, unknown>>;
    // Private channels by Mumble channel (a voice channel, or a text
    // channel's mirror): a lock.
    privateChannels?: Readonly<Record<number, unknown>>;
    // Voice channels being recorded: a red "REC".
    recording?: ReadonlySet<number>;
    // Live streams by id (see stream/protocol.streamId), and watching one.
    live?: ReadonlySet<number>;
    // Thumbnails of the streams (shown on the LIVE button).
    previews?: Readonly<Record<number, string>>;
    onWatch?: (session: number) => void;
    // Stored direct chats as listed, pinned ones first (see lib/directList;
    // mentions: unread messages).
    directs?: { channel: number; peer: Peer; pinned?: boolean }[];
    currentDirect?: number | null;
    onOpenDirect?: (peer: Peer) => void;
    onOpenText: (id: number) => void;
    onJoin: (id: number) => void;
    onUserMenu: (session: number, event: MouseEvent) => void;
    // A "+" in a category (undefined: the user may not create there).
    onCreate?: (kind: ChannelKind) => void;
    canCreate?: Record<ChannelKind, boolean>;
    // Right-click on a channel.
    onChannelMenu?: (kind: ChannelKind, id: number, event: MouseEvent) => void;
    // Right-click on a direct chat.
    onDirectMenu?: (channel: number, event: MouseEvent) => void;
  } = $props();

  // Text channel mirrors (for Mumble clients) are not voice channels here.
  const tree = $derived(withoutMirrors(channelTree(server), mirrorIds(server, textChannels)));
  let collapsed = $state(false);
  let directCollapsed = $state(false);
  let textCollapsed = $state(false);
  const INDENT = 16;
</script>

{#snippet channel(node: ChannelNode, depth: number)}
  {@const users = usersIn(server, node.channel.id)}
  <!-- Root is the lobby: its children sit at the same level, like Discord's
       flat list. Only deeper channels are indented. -->
  <li style:padding-left="{depth > 1 ? INDENT : 0}px">
    <button
      use:longPress
      class="channel hover-row"
      class:current={node.channel.id === currentChannel}
      aria-current={node.channel.id === currentChannel ? "page" : undefined}
      onclick={() => onJoin(node.channel.id)}
      oncontextmenu={(e) => {
        if (!onChannelMenu) return;
        e.preventDefault();
        onChannelMenu("voice", node.channel.id, e);
      }}
      title={htmlToText(node.channel.description) || undefined}
    >
      {#if node.channel.id in listenOnly}
        <Icon name="headphones" size={20} /><span class="sr-only">Listen only:</span>
      {:else}
        <Icon name="speaker" size={20} />
      {/if}
      <span class="name ellipsis">{node.channel.name}</span>
      {#if node.channel.id in privateChannels}{@render lock()}{/if}
      {#if recording.has(node.channel.id)}<span class="rec" title="This channel is being recorded">REC</span>{/if}
      {#if node.channel.id !== currentChannel}<Badge count={mentionVoice[node.channel.id]} label="mentions" />{/if}
    </button>
    {#if users.length}
      <ul class="users">
        {#each users as user (user.session)}
          <li>
            <button
              class="user hover-row"
              class:me={user.session === server.session}
              onclick={(e) => onUserMenu(user.session, e)}
              oncontextmenu={(e) => onUserMenu(user.session, e)}
            >
              <Avatar name={user.name} size={24} speaking={speaking.has(user.session)} />
              <span class="name ellipsis">{user.name}</span>
              <span class="flags">
                {#if localMuted.has(user.session)}
                  <span title="Muted for you">
                    <Icon name="speaker" off size={16} /><span class="sr-only">Muted for you</span>
                  </span>
                {/if}
                {#if user.selfMute || user.mute || user.suppress}
                  <span title="Muted">
                    <Icon name="mic" off size={16} /><span class="sr-only">Muted</span>
                  </span>
                {/if}
                {#if user.selfDeaf || user.deaf}
                  <span title="Deafened">
                    <Icon name="headphones" off size={16} /><span class="sr-only">Deafened</span>
                  </span>
                {/if}
                <Badge count={unread[user.name]} />
              </span>
            </button>
            {#each [streamId(user.session, false), streamId(user.session, true)].filter((id) => live.has(id)) as id (id)}
              {@const what = isCamera(id) ? "camera" : "screen"}
              {#if user.session === server.session}
                <span class="live" title="You are sharing your {what}">{@render badge(id)}</span>
              {:else}
                <span class="live-wrap">
                  <button class="live" title="Watch {user.name}'s {what}" aria-label="Watch {user.name}'s {what}" onclick={() => onWatch?.(id)}>{@render badge(id)}</button>
                  {#if previews[id]}
                    <img class="live-preview" src={previews[id]} alt="" />
                  {/if}
                </span>
              {/if}
            {/each}
          </li>
        {/each}
      </ul>
    {/if}
    {#if node.children.length}
      <ul>
        {#each node.children as child (child.channel.id)}
          {@render channel(child, depth + 1)}
        {/each}
      </ul>
    {/if}
  </li>
{/snippet}

{#snippet badge(id: number)}
  {#if isCamera(id)}<Icon name="camera" size={14} />{:else}LIVE{/if}
{/snippet}

{#snippet lock()}
  <span class="lock" title="Private channel"><Icon name="lock" size={14} /><span class="sr-only">(private)</span></span>
{/snippet}

{#snippet category(label: string, isCollapsed: boolean, toggle: () => void, kind?: ChannelKind)}
  <div class="category-row">
    <button class="category" aria-expanded={!isCollapsed} onclick={toggle}>
      <svg class="chevron" class:collapsed={isCollapsed} width="12" height="12" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M6 9l6 6 6-6" fill="none" stroke="currentColor" stroke-width="3" />
      </svg>
      {label}
    </button>
    {#if kind && onCreate && canCreate[kind]}
      <button class="add" title="Create channel" aria-label="Create {kind} channel" onclick={() => onCreate(kind)}>
        <Icon name="plus" size={16} />
      </button>
    {/if}
  </div>
{/snippet}

<nav aria-label="Channels">
  {#if textChannels.length || canCreate.text}
    {@render category("Text channels", textCollapsed, () => (textCollapsed = !textCollapsed), "text")}
    {#if !textCollapsed}
      <ul class="text-channels">
        {#each textChannels as ch (ch.id)}
          <li>
            <button
              use:longPress
              class="channel hover-row"
              class:current={ch.id === currentText}
              class:unread={unreadText.has(ch.id) && ch.id !== currentText}
              class:muted={mutedText.has(ch.id)}
              aria-current={ch.id === currentText ? "page" : undefined}
              onclick={() => onOpenText(ch.id)}
              oncontextmenu={(e) => {
                if (!onChannelMenu) return;
                e.preventDefault();
                onChannelMenu("text", ch.id, e);
              }}
            >
              <Icon name="hash" size={20} />
              <span class="name ellipsis">{ch.name}</span>
              {#if ch.mumble !== undefined && ch.mumble in privateChannels}{@render lock()}{/if}
              {#if unreadText.has(ch.id) && ch.id !== currentText}
                <span class="sr-only">unread</span>
              {/if}
              {#if ch.id !== currentText}<Badge count={mentionText[ch.id]} label="mentions" />{/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
  {@render category("Voice channels", collapsed, () => (collapsed = !collapsed), "voice")}
  {#if tree && !collapsed}
    {#if lobby}
      <ul>
        {#each tree.children as child (child.channel.id)}
          {@render channel(child, 1)}
        {/each}
      </ul>
    {:else}
      <ul>{@render channel(tree, 0)}</ul>
    {/if}
  {/if}
  {#if directs.length && onOpenDirect}
    {@render category("Direct messages", directCollapsed, () => (directCollapsed = !directCollapsed))}
    {#if !directCollapsed}
      <ul class="text-channels">
        {#each directs as d (d.channel)}
          <li>
            <button
              use:longPress
              class="channel hover-row"
              class:current={d.channel === currentDirect}
              class:unread={!!mentionText[d.channel] && d.channel !== currentDirect}
              class:muted={mutedText.has(d.channel)}
              aria-current={d.channel === currentDirect ? "page" : undefined}
              onclick={() => onOpenDirect(d.peer)}
              oncontextmenu={(e) => {
                if (!onDirectMenu) return;
                e.preventDefault();
                onDirectMenu(d.channel, e);
              }}
            >
              <Avatar name={d.peer.name} size={20} />
              <span class="name ellipsis">{d.peer.name}</span>
              {#if d.pinned}<span class="pinned" title="Pinned"><Icon name="pin" size={14} /><span class="sr-only">(pinned)</span></span>{/if}
              {#if d.channel !== currentDirect}<Badge count={mentionText[d.channel]} label="unread messages" />{/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
</nav>

<style>
  nav {
    flex: 1;
    overflow-y: auto;
    padding: 12px 8px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .category-row {
    display: flex;
    align-items: center;
  }
  .category-row .category {
    flex: 1;
  }
  .add {
    display: grid;
    place-items: center;
    padding: 0 4px 2px;
    color: var(--text-muted);
  }
  .add:hover {
    color: var(--text);
  }
  .category {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
    padding: 4px 4px 6px 2px;
    color: var(--text-muted);
    font-size: 12px;
    font-weight: 600;
  }
  .category:hover {
    color: var(--text);
  }
  .chevron {
    transition: transform 0.15s;
  }
  .chevron.collapsed {
    transform: rotate(-90deg);
  }
  .channel {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    /* A long press opens the channel menu, not the phone's text menu. */
    -webkit-touch-callout: none;
    user-select: none;
    height: 34px;
    margin: 1px 0;
    padding: 0 8px;
    border-radius: 6px;
    color: var(--text-muted);
    font-size: 15px;
    font-weight: 500;
    text-align: left;
  }
  .channel.current {
    background: var(--bg-active);
    color: var(--text-strong);
  }
  /* New messages: bright and bold, like Discord. */
  .channel.unread {
    color: var(--text-strong);
    font-weight: 600;
  }
  .channel.muted:not(.current) {
    opacity: 0.5;
  }
  .text-channels {
    margin-bottom: 16px;
  }
  .users {
    margin: 0 0 4px 28px;
  }
  .user {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    height: 32px;
    padding: 0 8px;
    border-radius: 6px;
    color: var(--text-muted);
    font-size: 14px;
    text-align: left;
  }
  .user.me {
    color: var(--text);
  }
  .name {
    flex: 1;
  }
  .rec {
    flex-shrink: 0;
    padding: 0 4px;
    border-radius: 4px;
    background: var(--danger);
    color: white;
    font-size: 10px;
    font-weight: 700;
  }
  .lock,
  .pinned {
    display: flex;
    flex-shrink: 0;
    color: var(--text-muted);
  }
  .flags {
    display: flex;
    align-items: center;
    gap: 4px;
    color: var(--text-faint);
  }
  .flags span {
    display: inline-flex;
  }
  .users li:has(.live) {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .users li:has(.live) .user {
    flex: 1;
    min-width: 0;
  }
  .live-wrap {
    position: relative;
    flex: none;
    margin-right: 8px;
  }
  .live-preview {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 20;
    display: none;
    width: 200px;
    border-radius: 6px;
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.5);
    pointer-events: none;
  }
  .live-wrap:hover .live-preview,
  .live-wrap:focus-within .live-preview {
    display: block;
  }
  span.live {
    margin-right: 8px;
  }
  .live {
    display: inline-flex;
    align-items: center;
    flex: none;
    padding: 0 5px;
    border-radius: 3px;
    background: var(--danger);
    color: #fff;
    font-size: 10px;
    font-weight: 700;
    line-height: 16px;
    cursor: pointer;
  }
</style>
