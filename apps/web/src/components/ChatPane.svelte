<script lang="ts">
  import type { ChatMessage } from "@kallisti/protocol";
  import { formatSize, type DisplayMessage } from "../lib/textChannels";
  import { chatItems, formatDay, formatTime, formatTimestamp } from "../lib/chat";
  import { nextScrollTop } from "../lib/scroll";
  import { untrack } from "svelte";
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
    more = false,
    loadingOlder = false,
    onLoadOlder,
    error = null,
    onUpload,
    uploading = false,
    onSend,
    onClose,
  }: {
    title: string;
    // channel: a Mumble voice channel's chat (not stored), text: a stored
    // Kallisti text channel, dm: a private conversation.
    kind: "channel" | "text" | "dm";
    messages: readonly (ChatMessage | DisplayMessage)[];
    welcomeText: string;
    membersOpen: boolean;
    onToggleMembers: () => void;
    // Text channels: older messages can be loaded.
    more?: boolean;
    loadingOlder?: boolean;
    onLoadOlder?: () => void;
    // The server refused something (for example a message that is too long).
    error?: string | null;
    // Text channels: attach a file (button, paste or drop).
    onUpload?: (file: File) => void;
    uploading?: boolean;
    // Missing: the conversation is read-only (the other user left).
    onSend?: (text: string) => void;
    onClose?: () => void;
  } = $props();

  let draft = $state("");
  let list: HTMLElement | undefined = $state();
  // "Today at" must turn into "Yesterday at" after midnight.
  let now = $state(Date.now());
  const items = $derived(chatItems(messages));
  const target = $derived(kind === "dm" ? `@${title}` : kind === "text" ? `#${title}` : title);

  $effect(() => {
    const timer = setInterval(() => (now = Date.now()), 60_000);
    return () => clearInterval(timer);
  });

  // Scrolling after new content: measure before the DOM update, then follow
  // the bottom or keep the reading position (see nextScrollTop). Tracks the
  // count, not `messages` (a new array on every server update); `target`
  // tells a conversation switch, the first message's time tells older
  // history loading above.
  // Both effects track exactly the same values, and the snapshot is used
  // once: a stale snapshot would move the reader back.
  let before: { scrollHeight: number; scrollTop: number; clientHeight: number } | null = null;
  let shown = { target: "", firstAt: 0 };
  $effect.pre(() => {
    void [messages.length, target];
    if (list) before = { scrollHeight: list.scrollHeight, scrollTop: list.scrollTop, clientHeight: list.clientHeight };
  });
  $effect(() => {
    void [messages.length, target];
    untrack(() => {
      if (!list) return;
      const firstAt = messages[0]?.at ?? 0;
      const switched = target !== shown.target || !before;
      const prepended = !switched && firstAt < shown.firstAt;
      list.scrollTop = before ? nextScrollTop(before, list.scrollHeight, switched, prepended) : list.scrollHeight;
      shown = { target, firstAt };
      before = null;
    });
  });

  let filePicker: HTMLInputElement | undefined = $state();
  // Counted: dragging over child elements fires enter/leave pairs.
  let dragDepth = $state(0);
  const dragging = $derived(dragDepth > 0);
  const sendFiles = (files: FileList | null | undefined) => {
    const file = files?.[0];
    if (file && onUpload) onUpload(file);
  };
  const onPaste = (event: ClipboardEvent) => {
    if (onUpload && event.clipboardData?.files.length) {
      event.preventDefault();
      sendFiles(event.clipboardData.files);
    }
  };
  const onDrop = (event: DragEvent) => {
    event.preventDefault();
    dragDepth = 0;
    sendFiles(event.dataTransfer?.files);
  };

  const submit = (event: SubmitEvent) => {
    event.preventDefault();
    const text = draft.trim();
    if (!text || !onSend) return;
    onSend(text);
    draft = "";
  };
</script>

<main
  class="chat"
  class:dragging
  ondragenter={(e) => {
    if (onUpload && e.dataTransfer?.types.includes("Files")) dragDepth++;
  }}
  ondragover={(e) => {
    if (onUpload && e.dataTransfer?.types.includes("Files")) e.preventDefault();
  }}
  ondragleave={() => {
    if (dragDepth > 0) dragDepth--;
  }}
  ondrop={onUpload ? onDrop : undefined}
>
  <header>
    {#if kind === "dm"}
      <span class="at" aria-hidden="true">@</span>
    {:else}
      <Icon name={kind === "text" ? "hash" : "speaker"} size={22} />
    {/if}
    <h1>{title}</h1>
    <div class="tools">
      {#if kind !== "dm"}
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
    {#if more}
      <div class="older">
        <button onclick={onLoadOlder} disabled={loadingOlder}>
          {loadingOlder ? "Loading…" : "Load older messages"}
        </button>
      </div>
    {:else}
    <div class="intro">
      <div class="intro-icon" aria-hidden="true">
        {#if kind === "dm"}@{:else}<Icon name={kind === "text" ? "hash" : "speaker"} size={36} />{/if}
      </div>
      <h2>{kind === "dm" ? title : `Welcome to ${target}!`}</h2>
      <p>
        {#if kind === "dm"}
          This is the start of your conversation with {title}. Private messages are not stored.
        {:else if kind === "text"}
          This is the start of the {target} channel.
        {:else}
          This is the start of the {title} voice channel's chat. These messages are not stored: you
          only see what was sent while you were here.
        {/if}
      </p>
      {#if welcomeText}
        <div class="welcome">{@html safe(welcomeText)}</div>
      {/if}
    </div>
    {/if}

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
                  {#if m.text}<div class="text">{@html safe(m.text)}</div>{/if}
                  {@render attachment(m)}
                </div>
              {:else}
                <div class="gutter">
                  <time class="hover-time" datetime={new Date(m.at).toISOString()}>{formatTime(m.at)}</time>
                </div>
                <div class="body">
                  {#if m.text}<div class="text">{@html safe(m.text)}</div>{/if}
                  {@render attachment(m)}
                </div>
              {/if}
            </li>
          {/each}
        {/if}
      {/each}
    </ol>
  </div>

  <form onsubmit={submit}>
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    <div class="composer" class:disabled={!onSend}>
      <button
        type="button"
        class="attach"
        title={onUpload ? "Upload a file" : "Uploads work in text channels"}
        aria-label="Upload a file"
        disabled={!onUpload || uploading}
        onclick={() => filePicker?.click()}
      >
        <Icon name="plus" size={18} />
      </button>
      <input
        type="file"
        class="sr-only"
        tabindex="-1"
        bind:this={filePicker}
        onchange={(e) => {
          sendFiles(e.currentTarget.files);
          e.currentTarget.value = "";
        }}
      />
      {#if uploading}<span class="uploading">Uploading…</span>{/if}
      <input
        onpaste={onPaste}
        bind:value={draft}
        placeholder={onSend ? `Message ${target}` : `${title} left the server`}
        disabled={!onSend}
        aria-label="Message"
      />
    </div>
  </form>
</main>

{#snippet attachment(m: ChatMessage | DisplayMessage)}
  {#if "upload" in m && m.upload}
    {#if m.upload.type.startsWith("image/")}
      <a class="image" href={m.upload.url} target="_blank" rel="noopener noreferrer">
        <img src={m.upload.url} alt={m.upload.name} loading="lazy" />
      </a>
    {:else}
      <a class="file" href={m.upload.url} rel="noopener noreferrer">
        <span class="file-name">{m.upload.name}</span>
        <span class="file-size">{formatSize(m.upload.size)}</span>
      </a>
    {/if}
  {/if}
{/snippet}

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
  .older {
    display: flex;
    justify-content: center;
    padding: 16px;
  }
  .older button {
    padding: 6px 14px;
    border-radius: 6px;
    background: var(--bg-input);
    color: var(--text-muted);
    font-size: 14px;
  }
  .older button:hover {
    color: var(--text);
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
  .error {
    margin: 0 0 8px;
    color: var(--danger);
    font-size: 14px;
  }
  .composer {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 0 12px 0 4px;
    border-radius: 8px;
    background: var(--bg-input);
  }
  .chat.dragging .scroller {
    outline: 2px dashed var(--accent);
    outline-offset: -8px;
  }
  .image {
    display: inline-block;
    margin-top: 4px;
  }
  .image img {
    display: block;
    max-width: min(400px, 100%);
    max-height: 300px;
    border-radius: 8px;
  }
  .file {
    display: inline-flex;
    flex-direction: column;
    gap: 2px;
    margin-top: 4px;
    padding: 10px 14px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg-sidebar);
    color: var(--link);
    text-decoration: none;
  }
  .file-size {
    color: var(--text-faint);
    font-size: 12px;
  }
  .uploading {
    color: var(--text-muted);
    font-size: 14px;
    white-space: nowrap;
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
