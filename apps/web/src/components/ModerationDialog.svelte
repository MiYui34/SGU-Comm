<script lang="ts">
  import { banAddress, type Ban, type Registered } from "@kallisti/protocol";
  import { untrack } from "svelte";

  // Server settings for moderators and admins: the ban list and the
  // registered users (the server checks every change again).
  let {
    bans,
    registered,
    rights,
    onLoad,
    onSetBans,
    onRename,
    onUnregister,
    onClose,
  }: {
    bans: readonly Ban[] | null;
    registered: readonly Registered[] | null;
    rights: { bans: boolean; editBans: boolean; users: boolean };
    onLoad: (what: "bans" | "users") => void;
    onSetBans: (bans: Ban[]) => void;
    onRename: (userId: number, name: string) => void;
    onUnregister: (userId: number) => void;
    onClose: () => void;
  } = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  let tab = $state<"bans" | "users">(untrack(() => (rights.bans ? "bans" : "users")));
  let renaming = $state<{ id: number; name: string } | null>(null);
  let confirming = $state<number | null>(null);
  $effect(() => dialog?.showModal());
  $effect(() => onLoad(tab));

  const duration = (seconds: number) =>
    seconds === 0 ? "for ever" : seconds >= 86400 ? `${Math.round(seconds / 86400)} days` : `${Math.round(seconds / 3600)} hours`;
</script>

<dialog bind:this={dialog} onclose={onClose} aria-labelledby="moderation-title">
  <header>
    <h2 id="moderation-title">Server settings</h2>
    <button type="button" class="close" onclick={() => (dialog?.close(), onClose())} aria-label="Close">×</button>
  </header>
  <div class="tabs" role="tablist">
    {#if rights.bans}
      <button role="tab" aria-selected={tab === "bans"} onclick={() => (tab = "bans")}>Bans</button>
    {/if}
    {#if rights.users}
      <button role="tab" aria-selected={tab === "users"} onclick={() => (tab = "users")}>Registered users</button>
    {/if}
  </div>

  {#if tab === "bans"}
    {#if bans === null}
      <p class="note">Loading…</p>
    {:else if bans.length === 0}
      <p class="note">Nobody is banned.</p>
    {:else}
      <ul>
        {#each bans as ban, i (i)}
          <li>
            <div class="what">
              <strong>{ban.name || banAddress(ban)}</strong>
              <span class="detail">
                {banAddress(ban)} · {duration(ban.duration)}{ban.start ? ` · since ${ban.start.slice(0, 10)}` : ""}
                {#if ban.reason} · {ban.reason}{/if}
              </span>
            </div>
            {#if rights.editBans}
              <button type="button" onclick={() => onSetBans(bans.filter((_, k) => k !== i))}>Remove</button>
            {/if}
          </li>
        {/each}
      </ul>
      {#if !rights.editBans}<p class="note">Only admins change the ban list.</p>{/if}
    {/if}
  {:else if registered === null}
    <p class="note">Loading…</p>
  {:else}
    <ul>
      {#each registered as user (user.userId)}
        <li>
          {#if renaming?.id === user.userId}
            <form
              class="rename"
              onsubmit={(e) => {
                e.preventDefault();
                if (renaming?.name.trim()) onRename(user.userId, renaming.name.trim());
                renaming = null;
              }}
            >
              <input bind:value={renaming.name} aria-label="New name for {user.name}" />
              <button type="submit">Save</button>
              <button type="button" onclick={() => (renaming = null)}>Cancel</button>
            </form>
          {:else}
            <div class="what"><strong>{user.name}</strong><span class="detail">#{user.userId}</span></div>
            {#if user.userId !== 0}
              {#if confirming === user.userId}
                <button type="button" class="danger" onclick={() => (onUnregister(user.userId), (confirming = null))}>
                  Unregister
                </button>
                <button type="button" onclick={() => (confirming = null)}>Cancel</button>
              {:else}
                <button type="button" onclick={() => (renaming = { id: user.userId, name: user.name })}>Rename</button>
                <button type="button" onclick={() => (confirming = user.userId)}>Unregister…</button>
              {/if}
            {/if}
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</dialog>

<style>
  dialog {
    width: min(560px, 94vw);
    max-height: 80vh;
    padding: 24px;
    border: none;
    border-radius: 8px;
    background: var(--bg-main);
    color: var(--text);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.6);
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
  }
  h2 {
    margin: 0;
    font-size: 18px;
  }
  .close {
    font-size: 24px;
    line-height: 1;
    color: var(--text-muted);
  }
  .tabs {
    display: flex;
    gap: 4px;
    margin-bottom: 12px;
  }
  .tabs button {
    padding: 6px 12px;
    border-radius: 4px;
    color: var(--text-muted);
  }
  .tabs button[aria-selected="true"] {
    background: var(--bg-active);
    color: var(--text);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 0;
    border-bottom: 1px solid var(--border-subtle);
  }
  .what {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .detail {
    font-size: 12px;
    color: var(--text-muted);
    overflow-wrap: anywhere;
  }
  li button {
    padding: 6px 10px;
    border-radius: 4px;
    background: var(--bg-input);
    color: var(--text);
  }
  li .danger {
    background: var(--danger);
    color: white;
  }
  .rename {
    display: flex;
    flex: 1;
    gap: 6px;
  }
  .rename input {
    flex: 1;
    padding: 6px 8px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
  }
  .note {
    color: var(--text-muted);
    font-size: 14px;
  }
</style>
