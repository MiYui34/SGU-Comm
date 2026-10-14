<script lang="ts">
  import type { User } from "@mumble-discord/protocol";
  import Avatar from "./Avatar.svelte";
  import Badge from "./Badge.svelte";

  let {
    here,
    elsewhere,
    channelName,
    unread,
    onUserMenu,
  }: {
    here: User[];
    elsewhere: User[];
    channelName: string;
    unread: Record<number, number>;
    onUserMenu: (session: number, event: MouseEvent) => void;
  } = $props();
</script>

{#snippet section(title: string, users: User[])}
  {#if users.length}
    <h2>{title} — {users.length}</h2>
    <ul>
      {#each users as user (user.session)}
        <li>
          <button
            onclick={(e) => onUserMenu(user.session, e)}
            oncontextmenu={(e) => onUserMenu(user.session, e)}
          >
            <Avatar name={user.name} size={32} />
            <span class="name">{user.name}</span>
            <span class="unread"><Badge count={unread[user.session]} /></span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
{/snippet}

<aside aria-label="Members">
  {@render section(`In ${channelName}`, here)}
  {@render section("Online", elsewhere)}
</aside>

<style>
  aside {
    overflow-y: auto;
    padding: 16px 8px;
    background: var(--bg-sidebar);
  }
  h2 {
    margin: 16px 8px 4px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  h2:first-child {
    margin-top: 0;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  button {
    display: flex;
    width: 100%;
    text-align: left;
    align-items: center;
    gap: 12px;
    padding: 6px 8px;
    border-radius: 4px;
    color: var(--text-muted);
  }
  button:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .unread {
    margin-left: auto;
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
