<script lang="ts">
  import type { ChatMessage } from "@mumble-discord/protocol";
  import { chatItems, formatDay, formatTime, formatTimestamp } from "../lib/chat";
  import { sanitize as safe } from "../lib/sanitize";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";

  let {
    title,
    kind,
    messages,
    welcomeText,
    membersOpen,
    onToggleMembers,
    onSend,
    onClose,
  }: {
    title: string;
    kind: "channel" | "dm";
    messages: readonly ChatMessage[];
    welcomeText: string;
    membersOpen: boolean;
    onToggleMembers: () => void;
    // Missing: the conversation is read-only (the other user left).
    onSend?: (text: string) => void;
    onClose?: () => void;
  } = $props();

  let draft = $state("");
  let list: HTMLElement | undefined = $state();
  // "Today at" must turn into "Yesterday at" after midnight.
  let now = $state(Date.now());
  const items = $derived(chatItems(messages));
  const target = $derived(kind === "dm" ? `@${title}` : title);

  $effect(() => {
    const timer = setInterval(() => (now = Date.now()), 60_000);
    return () => clearInterval(timer);
  });

  $effect(() => {
    // Not the number of groups: a new message can join the last group. Not
    // `messages` itself: it is a new array on every server update. The title
    // covers a channel switch to a channel with the same message count.
    void [messages.length, title];
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
      <Icon name="speaker" size={22} />
    {/if}
    <h1>{title}</h1>
    <div class="tools">
      {#if kind === "channel"}
        <button
          class:on={membersOpen}
          onclick={onToggleMembers}
          aria-pressed={membersOpen}
          title={membersOpen ? "Hide member list" : "Show member list"}
          aria-label="Member list"
          class="members-toggle"
        >
          <Icon name="members" size={22} />
        </button>
      {/if}
      {#if onClose}
        <button onclick={onClose} title="Close conversation" aria-label="Close conversation">
          <span class="x" aria-hidden="true">×</span>
        </button>
      {/if}
    </div>
  </header>

  <div class="scroller" bind:this={list}>
    <div class="intro">
      <div class="intro-icon" aria-hidden="true">
        {#if kind === "dm"}@{:else}<Icon name="speaker" size={36} />{/if}
      </div>
      <h2>{kind === "dm" ? title : `Welcome to ${title}!`}</h2>
      <p>
        {kind === "dm"
          ? `This is the start of your conversation with ${title}. Private messages are not stored.`
          : `This is the start of the ${title} channel. Messages are not stored: you only see what was sent while you were here.`}
      </p>
      {#if welcomeText}
        <div class="welcome">{@html safe(welcomeText)}</div>
      {/if}
    </div>

    <ol class="messages">
      {#each items as item (item.kind === "day" ? `day-${item.at}` : item.group.messages[0])}
        {#if item.kind === "day"}
          <li class="divider" role="separator"><span>{formatDay(item.at)}</span></li>
        {:else}
          {@const group = item.group}
          {#each group.messages as m, i (m)}
            <li class="line" class:first={i === 0}>
              {#if i === 0}
                <div class="gutter">
                  {#if group.actorName}
                    <Avatar name={group.actorName} size={40} />
                  {:else}
                    <span class="system-avatar"></span>
                  {/if}
                </div>
                <div class="body">
                  <div class="meta">
                    <span class="author">{group.actorName ?? "Server"}</span>
                    <time datetime={new Date(m.at).toISOString()}>{formatTimestamp(m.at, now)}</time>
                  </div>
                  <div class="text">{@html safe(m.text)}</div>
                </div>
              {:else}
                <div class="gutter">
                  <time class="hover-time" datetime={new Date(m.at).toISOString()}>{formatTime(m.at)}</time>
                </div>
                <div class="body">
                  <div class="text">{@html safe(m.text)}</div>
                </div>
              {/if}
            </li>
          {/each}
        {/if}
      {/each}
    </ol>
  </div>

  <form onsubmit={submit}>
    <div class="composer" class:disabled={!onSend}>
      <button type="button" class="attach" title="Uploads come later" disabled>
        <Icon name="plus" size={18} />
      </button>
      <input
        bind:value={draft}
        placeholder={onSend ? `Message ${target}` : `${title} left the server`}
        disabled={!onSend}
        aria-label="Message"
      />
    </div>
  </form>
</main>

<style>
  .chat {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    background: var(--bg-main);
  }
  header {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    gap: 8px;
    height: 48px;
    padding: 0 8px 0 16px;
    border-bottom: 1px solid var(--border);
    color: var(--text-faint);
  }
  .at {
    font-size: 22px;
    font-weight: 600;
  }
  h1 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--text);
  }
  .tools {
    display: flex;
    gap: 4px;
    margin-left: auto;
  }
  .tools button {
    display: inline-flex;
    padding: 4px;
    border-radius: 6px;
    color: var(--text-muted);
  }
  .tools button:hover,
  .tools button.on {
    color: var(--text);
  }
  .x {
    width: 22px;
    font-size: 24px;
    line-height: 22px;
  }
  .scroller {
    flex: 1;
    overflow-y: auto;
    padding-bottom: 16px;
  }
  .intro {
    padding: 32px 16px 8px;
  }
  .intro-icon {
    display: grid;
    width: 68px;
    height: 68px;
    place-items: center;
    border-radius: 50%;
    background: var(--bg-active);
    color: var(--text);
    font-size: 36px;
    font-weight: 600;
  }
  h2 {
    margin: 8px 0;
    font-size: 30px;
    font-weight: 700;
  }
  .intro p {
    margin: 0;
    color: var(--text-muted);
  }
  .welcome {
    margin-top: 16px;
    padding: 12px 16px;
    border-left: 4px solid var(--accent);
    border-radius: 4px;
    background: var(--bg-sidebar);
    color: var(--text-muted);
  }
  .messages {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .divider {
    display: flex;
    align-items: center;
    margin: 24px 16px 8px;
    color: var(--text-faint);
    font-size: 12px;
    font-weight: 600;
  }
  .divider::before,
  .divider::after {
    flex: 1;
    height: 1px;
    background: var(--border-subtle);
    content: "";
  }
  .divider span {
    padding: 0 8px;
  }
  .line {
    display: flex;
    padding: 1px 48px 1px 0;
  }
  .line.first {
    margin-top: 16px;
  }
  .line:hover {
    background: rgb(0 0 0 / 0.08);
  }
  .gutter {
    display: flex;
    flex-shrink: 0;
    justify-content: center;
    width: 72px;
    padding-top: 2px;
  }
  .hover-time {
    visibility: hidden;
    font-size: 11px;
    line-height: 22px;
    color: var(--text-faint);
  }
  .line:hover .hover-time {
    visibility: visible;
  }
  .system-avatar {
    width: 40px;
    height: 40px;
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
    font-size: 16px;
    font-weight: 600;
    color: var(--text);
  }
  .meta time {
    font-size: 12px;
    color: var(--text-faint);
  }
  .text {
    font-size: 16px;
    line-height: 1.375;
    overflow-wrap: anywhere;
  }
  .text :global(a) {
    color: var(--link);
  }
  .text :global(img) {
    max-width: 400px;
    border-radius: 8px;
  }
  form {
    flex-shrink: 0;
    padding: 0 16px 24px;
  }
  .composer {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 0 12px 0 4px;
    border-radius: 8px;
    background: var(--bg-input);
  }
  .composer:focus-within {
    outline: 2px solid var(--accent);
  }
  .composer.disabled {
    opacity: 0.6;
  }
  .attach {
    display: grid;
    width: 24px;
    height: 24px;
    margin: 10px 8px 10px 8px;
    place-items: center;
    border-radius: 50%;
    background: var(--text-muted);
    color: var(--bg-input);
  }
  input {
    flex: 1;
    min-width: 0;
    height: 44px;
    border: none;
    background: none;
    font-size: 16px;
    outline: none;
  }
  input::placeholder {
    color: var(--text-faint);
  }
  /* The member list is hidden on narrow screens: so is its toggle. */
  @media (max-width: 900px) {
    .tools .members-toggle {
      display: none;
    }
  }
</style>
