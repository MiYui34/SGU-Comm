<script lang="ts">
  import type { Role } from "../lib/roles";
  import type { UserStatus } from "../lib/presence";
  import type { User } from "@kallisti/protocol";
  import type { Peer } from "../lib/textChannels";
  import Avatar from "./Avatar.svelte";
  import Badge from "./Badge.svelte";
  import Emoji from "./Emoji.svelte";
  import RoleTag from "./RoleTag.svelte";

  let {
    here,
    elsewhere,
    channelName,
    unread,
    speaking,
    onUserMenu,
    offline = [],
    onOfflineMenu = () => {},
    roleOf = () => null,
    statusOf = () => ({ presence: "online", custom: null }),
    inert = false,
  }: {
    here: User[];
    elsewhere: User[];
    channelName: string;
    // Unread private messages per user name.
    unread: Record<string, number>;
    speaking: ReadonlySet<number>;
    onUserMenu: (session: number, event: MouseEvent) => void;
    // Registered users who are offline (see lib/members), by user id.
    offline?: Peer[];
    onOfflineMenu?: (userId: number, event: MouseEvent) => void;
    roleOf?: (user: Pick<User, "userId">) => Role | null;
    statusOf?: (user: User) => UserStatus;
    inert?: boolean;
  } = $props();
</script>

<!-- Offline users: faded, no status (Discord hides their custom one). -->
{#snippet row(name: string, role: Role | null, status: UserStatus | null, speaking: boolean, onMenu: (e: MouseEvent) => void)}
  <li>
    <button class="hover-row" class:offline={!status} onclick={onMenu} oncontextmenu={onMenu}>
      <Avatar {name} size={32} status={status?.presence} {speaking} registered={!status} />
      <span class="who">
        <span class="line">
          <span class="ellipsis">{name}</span>
          <RoleTag {role} />
        </span>
        {#if status?.custom}
          <span class="custom ellipsis" title={status.custom.text}><Emoji text={status.custom.emoji} lazy /> {status.custom.text}</span>
        {/if}
      </span>
      <span class="unread"><Badge count={unread[name]} /></span>
    </button>
  </li>
{/snippet}

{#snippet section(title: string, users: User[])}
  {#if users.length}
    <h2>{title} — {users.length}</h2>
    <ul>
      {#each users as user (user.session)}
        {@render row(user.name, roleOf(user), statusOf(user), speaking.has(user.session), (e) => onUserMenu(user.session, e))}
      {/each}
    </ul>
  {/if}
{/snippet}

<aside aria-label="Members" {inert}>
  {@render section(`In ${channelName}`, here)}
  {@render section("Online", elsewhere)}
  {#if offline.length}
    <h2>Offline — {offline.length}</h2>
    <ul>
      {#each offline as p (p.id)}
        {@render row(p.name, roleOf({ userId: p.id }), null, false, (e) => onOfflineMenu(p.id, e))}
      {/each}
    </ul>
  {/if}
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
  .who {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .line {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }
  .custom {
    font-size: 12px;
    font-weight: 400;
  }
  .unread {
    margin-left: auto;
  }
  .offline {
    opacity: 0.4;
  }
  .offline:hover {
    opacity: 1;
  }
</style>
