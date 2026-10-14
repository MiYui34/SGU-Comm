<script lang="ts">
  import { channelTree, usersIn, type ChannelNode, type ServerState } from "@mumble-discord/protocol";
  import Avatar from "./Avatar.svelte";
  import Badge from "./Badge.svelte";
  import Icon from "./Icon.svelte";

  let {
    server,
    currentChannel,
    speaking,
    localMuted,
    unread,
    onJoin,
    onUserMenu,
  }: {
    server: ServerState;
    currentChannel: number | null;
    speaking: ReadonlySet<number>;
    localMuted: ReadonlySet<number>;
    unread: Record<number, number>;
    onJoin: (id: number) => void;
    onUserMenu: (session: number, event: MouseEvent) => void;
  } = $props();

  const tree = $derived(channelTree(server));
  let collapsed = $state(false);
  const INDENT = 16;
</script>

{#snippet channel(node: ChannelNode, depth: number)}
  {@const users = usersIn(server, node.channel.id)}
  <!-- Root is the lobby: its children sit at the same level, like Discord's
       flat list. Only deeper channels are indented. -->
  <li style:padding-left="{depth > 1 ? INDENT : 0}px">
    <button
      class="channel"
      class:current={node.channel.id === currentChannel}
      onclick={() => onJoin(node.channel.id)}
      title={node.channel.description || undefined}
    >
      <Icon name="speaker" size={20} />
      <span class="name">{node.channel.name}</span>
    </button>
    {#if users.length}
      <ul class="users">
        {#each users as user (user.session)}
          <li>
            <button
              class="user"
              class:me={user.session === server.session}
              onclick={(e) => onUserMenu(user.session, e)}
              oncontextmenu={(e) => onUserMenu(user.session, e)}
            >
              <Avatar name={user.name} size={24} speaking={speaking.has(user.session)} />
              <span class="name">{user.name}</span>
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
                <Badge count={unread[user.session]} />
              </span>
            </button>
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

<nav aria-label="Channels">
  <button class="category" aria-expanded={!collapsed} onclick={() => (collapsed = !collapsed)}>
    <svg class="chevron" class:collapsed width="12" height="12" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M6 9l6 6 6-6" fill="none" stroke="currentColor" stroke-width="3" />
    </svg>
    Voice channels
  </button>
  {#if tree && !collapsed}
    <ul>{@render channel(tree, 0)}</ul>
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
    height: 34px;
    margin: 1px 0;
    padding: 0 8px;
    border-radius: 6px;
    color: var(--text-muted);
    font-size: 15px;
    font-weight: 500;
    text-align: left;
  }
  .channel:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .channel.current {
    background: var(--bg-active);
    color: white;
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
  .user:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .user.me {
    color: var(--text);
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
</style>
