<script lang="ts">
  import type { Badge as Tag } from "../lib/roles";
  import type { UserStatus } from "../lib/presence";
  import type { User } from "@kallisti/protocol";
  import type { Peer } from "../lib/textChannels";
  import { onlineRows, type OnlineMember, type ReaderGroup, type ReaderRow } from "../lib/members";
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
    hidden = [],
    readers = null,
    voiceFirst = false,
    onMemberMenu = () => {},
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
    // Registered users who are offline, and members online in channels we
    // cannot see (see lib/members): their menu goes by user id.
    offline?: Peer[];
    hidden?: OnlineMember[];
    // In a private channel: who reads it, instead of the groups above.
    readers?: ReaderGroup<User>[] | null;
    // A voice channel's view: who is in it stays on top of the readers.
    voiceFirst?: boolean;
    onMemberMenu?: (userId: number, event: MouseEvent) => void;
    roleOf?: (user: Pick<User, "userId">) => Tag | null;
    statusOf?: (session: number) => UserStatus;
    inert?: boolean;
  } = $props();
</script>

<!-- Offline users: faded, no status (Discord hides their custom one). -->
{#snippet row(name: string, role: Tag | null, status: UserStatus | null, speaking: boolean, onMenu: (e: MouseEvent) => void)}
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

<!-- A group: users we see, members online in channels we cannot see
     (without a channel), offline members. -->
{#snippet rows(title: string, list: ReaderRow<User>[])}
  {#if list.length}
    <h2>{title} — {list.length}</h2>
    <ul>
      {#each list as r ("user" in r ? `s${r.user.session}` : "member" in r ? `u${r.member.id}` : `o${r.offline.id}`)}
        {#if "user" in r}
          {@const user = r.user}
          {@render row(user.name, roleOf(user), statusOf(user.session), speaking.has(user.session), (e) => onUserMenu(user.session, e))}
        {:else if "member" in r}
          {@const p = r.member}
          {@render row(p.name, roleOf({ userId: p.id }), statusOf(p.session), false, (e) => onMemberMenu(p.id, e))}
        {:else}
          {@const p = r.offline}
          {@render row(p.name, roleOf({ userId: p.id }), null, false, (e) => onMemberMenu(p.id, e))}
        {/if}
      {/each}
    </ul>
  {/if}
{/snippet}

<aside aria-label="Members" {inert}>
  {#if readers}
    {#if voiceFirst}{@render rows(`In ${channelName}`, onlineRows(here, []))}{/if}
    {#each readers as g (g.title)}
      {@render rows(g.title, g.rows)}
    {/each}
  {:else}
    {@render rows(`In ${channelName}`, onlineRows(here, []))}
    {@render rows("Online", onlineRows(elsewhere, hidden))}
    {@render rows("Offline", offline.map((p) => ({ offline: p })))}
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
