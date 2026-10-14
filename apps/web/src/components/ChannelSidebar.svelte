<script lang="ts">
  import { channelTree, usersIn, type ChannelNode, type ServerState } from "@mumble-discord/protocol";
  import Avatar from "./Avatar.svelte";
  import Badge from "./Badge.svelte";
  import Icon from "./Icon.svelte";

  let {
    state,
    currentChannel,
    speaking,
    localMuted,
    unread,
    onJoin,
    onUserMenu,
  }: {
    state: ServerState;
    currentChannel: number | null;
    speaking: ReadonlySet<number>;
    localMuted: ReadonlySet<number>;
    unread: Record<number, number>;
    onJoin: (id: number) => void;
    onUserMenu: (session: number, event: MouseEvent) => void;
  } = $props();

  const tree = $derived(channelTree(state));
</script>

{#snippet channel(node: ChannelNode, depth: number)}
  <li>
    <button
      class="channel"
      class:current={node.channel.id === currentChannel}
      style:padding-left="{8 + depth * 14}px"
      onclick={() => onJoin(node.channel.id)}
      title={node.channel.description || node.channel.name}
    >
      <Icon name="speaker" />
      <span class="name">{node.channel.name}</span>
    </button>
    <ul class="users" style:padding-left="{30 + depth * 14}px">
      {#each usersIn(state, node.channel.id) as user (user.session)}
        <li
          class="user"
          class:me={user.session === state.session}
          oncontextmenu={(e) => onUserMenu(user.session, e)}
        >
          <Avatar name={user.name} size={22} speaking={speaking.has(user.session)} />
          <span class="name">{user.name}</span>
          {#if user.selfMute || user.mute || user.suppress}
            <span class="flag"><Icon name="mic" off size={14} /></span>
          {/if}
          {#if user.selfDeaf || user.deaf}
            <span class="flag"><Icon name="headphones" off size={14} /></span>
          {/if}
          {#if localMuted.has(user.session)}
            <span class="flag" title="Muted for you"><Icon name="speaker" off size={14} /></span>
          {/if}
          <Badge count={unread[user.session]} />
        </li>
      {/each}
    </ul>
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
  {#if tree}
    <ul>{@render channel(tree, 0)}</ul>
  {/if}
</nav>

<style>
  nav {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .channel {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 6px 8px;
    border-radius: 4px;
    color: var(--text-muted);
    text-align: left;
  }
  .channel:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .channel.current {
    background: var(--bg-active);
    color: var(--text);
  }
  .user {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 3px 0;
    color: var(--text-muted);
    font-size: 14px;
  }
  .user.me {
    color: var(--text);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .flag {
    display: inline-flex;
    color: var(--text-faint);
  }
  .user .name {
    flex: 1;
  }
</style>
