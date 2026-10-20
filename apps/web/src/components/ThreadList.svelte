<script lang="ts">
  import { replyBoxText, type Thread } from "../lib/threads";
  import Badge from "./Badge.svelte";
  import Icon from "./Icon.svelte";

  // A text channel's threads, in place of the member list: the active
  // ones, then the archived ones page by page (see lib/threads threadList).
  let {
    channel,
    threads,
    more,
    unread,
    mentions,
    onOpen,
    onMore,
    onClose,
    inert = false,
  }: {
    channel: string;
    threads: readonly Thread[];
    // Older archived threads: true (a button), false (none), null (loading).
    more: boolean | null;
    unread: ReadonlySet<number>;
    mentions: Readonly<Record<number, number>>;
    onOpen: (t: Thread) => void;
    onMore: () => void;
    onClose: () => void;
    inert?: boolean; // in a closed phone drawer
  } = $props();
  const now = Date.now();
  let closeButton: HTMLButtonElement | undefined = $state();
  $effect(() => closeButton?.focus());
</script>

<aside class="threads" aria-label="Threads" {inert}>
  <header>
    <span class="ellipsis">Threads in #{channel}</span>
    <button type="button" aria-label="Close threads" bind:this={closeButton} onclick={onClose}>×</button>
  </header>
  {#if threads.length === 0 && more === false}
    <p class="none">No threads here yet. Start one from a message.</p>
  {:else}
    <ul>
      {#each threads as t (t.id)}
        <li>
          <button type="button" class="hover-row" class:unread={unread.has(t.id)} onclick={() => onOpen(t)}>
            <Icon name="thread" size={18} />
            <span class="body">
              <span class="name ellipsis">{t.name}</span>
              <span class="meta ellipsis">{t.archived ? "Archived · " : ""}{replyBoxText(t, now)}</span>
            </span>
            <Badge count={mentions[t.id]} label="mentions" />
          </button>
        </li>
      {/each}
    </ul>
  {/if}
  {#if more !== false}
    <div class="more">
      <button type="button" disabled={more === null} onclick={onMore}>{more === null ? "Loading…" : "Older archived threads"}</button>
    </div>
  {/if}
</aside>

<style>
  .threads {
    overflow-y: auto;
    padding: 8px;
    background: var(--bg-main);
    border-left: 1px solid var(--border);
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 4px 8px 8px;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-muted);
  }
  header button {
    font-size: 20px;
    line-height: 1;
    color: var(--text-muted);
  }
  .none {
    padding: 8px;
    color: var(--text-muted);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li button {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px;
    border-radius: 6px;
    text-align: left;
    color: var(--text-muted);
  }
  li button.unread {
    color: var(--text);
    font-weight: 600;
  }
  .body {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .name {
    font-size: 14px;
    color: var(--text);
  }
  .meta {
    font-size: 12px;
    font-weight: 400;
  }
  .more {
    padding: 8px;
    text-align: center;
  }
  .more button {
    padding: 6px 12px;
    border-radius: 4px;
    background: var(--bg-input);
    color: var(--text);
    font-size: 13px;
  }
</style>
