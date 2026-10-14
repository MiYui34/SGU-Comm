<script lang="ts">
  import type { User } from "@kallisti/protocol";
  import Avatar from "./Avatar.svelte";
  import Badge from "./Badge.svelte";

  let {
    here,
    elsewhere,
    channelName,
    unread,
    speaking,
    onUserMenu,
    inert = false,
  }: {
    here: User[];
    elsewhere: User[];
    channelName: string;
    // Unread private messages per user name.
    unread: Record<string, number>;
    speaking: ReadonlySet<number>;
    onUserMenu: (session: number, event: MouseEvent) => void;
    inert?: boolean;
  } = $props();
</script>

{#snippet section(title: string, users: User[])}
  {#if users.length}
    <h2>{title} — {users.length}</h2>
    <ul>
      {#each users as user (user.session)}
        <li>
          <button
            class="hover-row"
            onclick={(e) => onUserMenu(user.session, e)}
            oncontextmenu={(e) => onUserMenu(user.session, e)}
          >
            <Avatar name={user.name} size={32} online speaking={speaking.has(user.session)} />
            <span class="ellipsis">{user.name}</span>
            <span class="unread"><Badge count={unread[user.name]} /></span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
{/snippet}

<aside aria-label="Members" {inert}>
  {@render section(`In ${channelName}`, here)}
  {@render section("Online", elsewhere)}
</aside>

<style>
  aside {
    --avatar-cutout: var(--bg-main);
    overflow-y: auto;
    padding: 12px 8px;
    background: var(--bg-main);
  }
  h2 {
    margin: 20px 8px 4px;
    font-size: 12px;
    font-weight: 600;
    letter-spacing: 0.02em;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  h2:first-child {
    margin-top: 8px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  button {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    height: 42px;
    margin: 1px 0;
    padding: 0 8px;
    border-radius: 6px;
    color: var(--text-muted);
    font-size: 15px;
    font-weight: 500;
    text-align: left;
  }
  button:hover {
    --avatar-cutout: var(--bg-hover);
  }
  .unread {
    margin-left: auto;
  }
</style>
