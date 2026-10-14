<script lang="ts">
  import type { ChatMessage } from "@mumble-discord/protocol";
  import { formatTime, groupMessages } from "../lib/chat";
  import { sanitize as safe } from "../lib/sanitize";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";

  let {
    title,
    kind,
    messages,
    welcomeText,
    onSend,
    onClose,
  }: {
    title: string;
    kind: "channel" | "dm";
    messages: readonly ChatMessage[];
    welcomeText: string;
    // Missing: the conversation is read-only (the other user left).
    onSend?: (text: string) => void;
    onClose?: () => void;
  } = $props();

  const target = $derived(kind === "dm" ? `@${title}` : title);

  let draft = $state("");
  let list: HTMLElement | undefined = $state();
  const groups = $derived(groupMessages(messages));

  $effect(() => {
    // Not groups.length: a new message can join the last group.
    void messages.length;
    list?.scrollTo({ top: list.scrollHeight });
  });

  const submit = (event: SubmitEvent) => {
    event.preventDefault();
    const text = draft.trim();
    if (!text || !onSend) return;
    onSend(text);
    draft = "";
  };
</script>

<main class="chat">
  <header>
    {#if kind === "dm"}
      <span class="at" aria-hidden="true">@</span>
    {:else}
      <Icon name="speaker" size={20} />
    {/if}
    <h1>{title}</h1>
    {#if onClose}
      <button class="close" onclick={onClose} aria-label="Close conversation">×</button>
    {/if}
  </header>

  <ol class="messages" bind:this={list}>
    {#if welcomeText}
      <li class="welcome">{@html safe(welcomeText)}</li>
    {/if}
    {#each groups as group (group.messages[0])}
      <li class="group">
        {#if group.actorName}
          <Avatar name={group.actorName} size={40} />
        {:else}
          <span class="system-avatar"></span>
        {/if}
        <div class="body">
          <div class="meta">
            <span class="author">{group.actorName ?? "Server"}</span>
            <time>{formatTime(group.at)}</time>
          </div>
          {#each group.messages as m (m)}
            <div class="text">{@html safe(m.text)}</div>
          {/each}
        </div>
      </li>
    {/each}
  </ol>

  <form onsubmit={submit}>
    <input
      bind:value={draft}
      placeholder={onSend ? `Message ${target}` : `${title} left the server`}
      disabled={!onSend}
      aria-label="Message"
    />
  </form>
</main>

<style>
  .chat {
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg-main);
  }
  header {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 48px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border);
    color: var(--text-muted);
  }
  .at {
    font-size: 20px;
    font-weight: 600;
  }
  .close {
    margin-left: auto;
    font-size: 22px;
    color: var(--text-muted);
  }
  .close:hover {
    color: var(--text);
  }
  h1 {
    margin: 0;
    font-size: 16px;
    color: var(--text);
  }
  .messages {
    flex: 1;
    overflow-y: auto;
    list-style: none;
    margin: 0;
    padding: 16px 0;
  }
  .welcome {
    margin: 0 16px 16px;
    padding: 12px;
    border-radius: 8px;
    background: var(--bg-sidebar);
    color: var(--text-muted);
  }
  .group {
    display: flex;
    gap: 16px;
    padding: 4px 16px;
    margin-top: 12px;
  }
  .group:hover {
    background: var(--bg-hover);
  }
  .system-avatar {
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: 50%;
    background: var(--bg-active);
  }
  .body {
    min-width: 0;
  }
  .meta {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .author {
    font-weight: 600;
  }
  time {
    font-size: 12px;
    color: var(--text-faint);
  }
  .text {
    line-height: 1.4;
    overflow-wrap: anywhere;
  }
  .text :global(img) {
    max-width: 400px;
    border-radius: 4px;
  }
  form {
    padding: 0 16px 24px;
  }
  input {
    width: 100%;
    padding: 12px 16px;
    border: none;
    border-radius: 8px;
    background: var(--bg-input);
    outline: none;
  }
</style>
