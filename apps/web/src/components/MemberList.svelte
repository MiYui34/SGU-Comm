<script lang="ts">
  import type { User } from "@mumble-discord/protocol";
  import Avatar from "./Avatar.svelte";

  let {
    here,
    elsewhere,
    channelName,
  }: { here: User[]; elsewhere: User[]; channelName: string } = $props();
</script>

{#snippet section(title: string, users: User[])}
  {#if users.length}
    <h2>{title} — {users.length}</h2>
    <ul>
      {#each users as user (user.session)}
        <li>
          <Avatar name={user.name} size={32} />
          <span>{user.name}</span>
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
  li {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 6px 8px;
    border-radius: 4px;
    color: var(--text-muted);
  }
  li:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
