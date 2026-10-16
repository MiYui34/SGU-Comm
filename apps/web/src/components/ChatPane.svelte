<script lang="ts">
  import { autosize } from "../lib/autosize";
  import { opensLightbox } from "../lib/lightbox";
  import { COMMON_EMOJI, reactedBy } from "../lib/reactions";
  import type { LinkPreview } from "../lib/linkPreview";
  import { highlightMentions, insertMention, mentionQuery, pingsMe, suggestNames } from "../lib/mentions";
  import Lightbox from "./Lightbox.svelte";
  import { escapeHtml, type ChatMessage } from "@kallisti/protocol";
  import { formatSize, type DisplayMessage } from "../lib/textChannels";
  import { chatItems, formatDay, formatTime, formatTimestamp } from "../lib/chat";
  import { nextScrollTop } from "../lib/scroll";
  import { tick, untrack } from "svelte";
  import { isChannelLink, sanitize as safe } from "../lib/sanitize";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";

  let {
    title,
    topic = "",
    kind,
    stored = false,
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
    onMenu,
    menuOpen = false,
    onHoldTalk,
    talking = false,
    onSend,
    onClose,
    me = "",
    people = [],
    onSearch,
    onPins,
    onOpenChannel,
    pinsOpen = false,
    mayPin = false,
    onPin,
    jumpTarget = null,
    onJumped,
    mayEdit,
    mayDelete,
    onEdit,
    onDelete,
    typing = "",
    onTyping,
    onReact,
    myId = null,
    previewOf,
    httpBase = "",
  }: {
    title: string;
    // Shown after the title, like a Discord channel topic (plain text).
    topic?: string;
    // channel: a Mumble voice channel's chat (not stored), text: a stored
    // Kallisti text channel, dm: a private conversation.
    kind: "channel" | "text" | "dm";
    // A voice channel's chat that the server stores (with history).
    stored?: boolean;
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
    // Phones: open the channel list, and push-to-talk as a hold button.
    onMenu?: () => void;
    menuOpen?: boolean;
    onHoldTalk?: (down: boolean) => void;
    // Really sending (also false after a window blur released the key).
    talking?: boolean;
    // Our name: messages that mention us stand out.
    me?: string;
    // Names "@" suggests (online users, recent authors).
    people?: readonly string[];
    // Searching stored messages; jumpTarget: a message to scroll to once
    // it is loaded (then onJumped).
    onSearch?: (text: string) => void;
    // A "#name" link in a message was clicked.
    onOpenChannel?: (id: number) => void;
    // The pinned messages panel (undefined: none here).
    onPins?: () => void;
    pinsOpen?: boolean;
    mayPin?: boolean;
    onPin?: (id: number, on: boolean) => void;
    jumpTarget?: number | null;
    onJumped?: () => void;
    // Stored messages: who may change which (the server checks again).
    mayEdit?: (m: DisplayMessage) => boolean;
    mayDelete?: (m: DisplayMessage) => boolean;
    onEdit?: (id: number, text: string) => void;
    onDelete?: (id: number) => void;
    // "bob is typing…" under the message box; onTyping: we type.
    typing?: string;
    onTyping?: () => void;
    // Stored messages: add (on) or remove our reaction; myId: our Mumble
    // user id (null: not registered, no reactions).
    onReact?: (id: number, emoji: string, on: boolean) => void;
    myId?: number | null;
    // Link previews of stored messages; images are on the server (httpBase).
    previewOf?: (id: number) => LinkPreview | null;
    httpBase?: string;
    // Text channels: attach a file (button, paste or drop), with a caption.
    onUpload?: (file: File, caption: string) => void;
    uploading?: boolean;
    // Missing: the conversation is read-only (the other user left).
    // reply: the stored message it answers (stored chats only).
    onSend?: (text: string, reply?: number) => void;
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
  // Images load after the scroll and make the content taller. A reader at
  // the bottom stays there.
  let atBottom = true;
  const trackBottom = () => {
    if (list) atBottom = list.scrollHeight - list.scrollTop - list.clientHeight <= 80;
  };
  const keepBottom = () => {
    if (list && atBottom) list.scrollTop = list.scrollHeight;
  };
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
      trackBottom();
    });
  });

  let filePicker: HTMLInputElement | undefined = $state();
  // Counted: dragging over child elements fires enter/leave pairs.
  let dragDepth = $state(0);
  const dragging = $derived(dragDepth > 0);
  // A chosen file waits above the message box until it is sent with Enter
  // (with the typed text as caption) or cancelled.
  let pending = $state<{ file: File; preview: string | null } | null>(null);
  const cancelPending = () => {
    if (pending?.preview) URL.revokeObjectURL(pending.preview);
    pending = null;
  };
  const sendFiles = (files: FileList | null | undefined) => {
    const file = files?.[0];
    if (!file || !onUpload) return;
    cancelPending();
    pending = { file, preview: file.type.startsWith("image/") ? URL.createObjectURL(file) : null };
  };
  // Another conversation: drop a waiting file.
  $effect(() => {
    void target;
    untrack(cancelPending);
  });
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

  let held = false;
  const release = () => {
    if (!held) return;
    held = false;
    onHoldTalk?.(false);
  };
  // The button can disappear while held (wider screen, mode change): the
  // microphone must not stay on.
  let lastHold: typeof onHoldTalk;
  $effect(() => {
    const hold = onHoldTalk;
    lastHold = hold;
    return () => {
      if (held && !lastHold) {
        held = false;
        hold?.(false);
      }
    };
  });
  $effect(() => () => {
    if (held) onHoldTalk?.(false);
  });

  const submit = (event: SubmitEvent) => {
    event.preventDefault();
    const text = draft.trim();
    if (pending && onUpload) {
      onUpload(pending.file, text);
      cancelPending();
      draft = "";
      return;
    }
    if (!text || !onSend) return;
    onSend(text, replying?.id);
    replying = null;
    draft = "";
  };

  // The image shown large (see Lightbox).
  let lightbox = $state<{ url: string; name: string } | null>(null);

  // "@" autocomplete (see mentionQuery). Escape hides the list until the
  // "@word" changes.
  let messageInput: HTMLTextAreaElement | undefined = $state();
  let caret = $state(0);
  let selected = $state(0);
  let dismissed = $state<number | null>(null);
  const listId = $props.id();
  const query = $derived(mentionQuery(draft, caret));
  const suggestions = $derived(query && query.start !== dismissed ? suggestNames(people, query.query, me) : []);
  const active = $derived(Math.min(selected, suggestions.length - 1));
  const updateCaret = () => {
    caret = messageInput?.selectionStart ?? draft.length;
    selected = 0;
    if (!query || query.start !== dismissed) dismissed = null;
  };
  const pick = async (name: string) => {
    if (!query) return;
    const next = insertMention(draft, query.start, caret, name);
    draft = next.text;
    caret = next.caret;
    await tick();
    messageInput?.focus();
    messageInput?.setSelectionRange(next.caret, next.caret);
  };
  // Keys while the list is open; true: handled.
  const onSuggestionKey = (e: KeyboardEvent) => {
    const n = suggestions.length;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") selected = (active + (e.key === "ArrowDown" ? 1 : n - 1)) % n;
    else if (e.key === "Enter" || e.key === "Tab") void pick(suggestions[active]!);
    else if (e.key === "Escape") dismissed = query?.start ?? null;
    else return false;
    e.preventDefault();
    return true;
  };

  // Editing and deleting stored messages.
  let editing = $state<{ id: number; text: string; old: string; upload: boolean } | null>(null);
  let confirming = $state<number | null>(null);
  let editInput: HTMLTextAreaElement | undefined = $state();
  let cancelButton: HTMLButtonElement | undefined = $state();
  const startEdit = async (m: ChatMessage | DisplayMessage) => {
    if (!("source" in m) || !m.source) return;
    editing = { id: m.source.id, text: m.source.text, old: m.source.text, upload: !!("upload" in m && m.upload) };
    await tick();
    editInput?.focus();
  };
  const saveEdit = () => {
    if (!editing) return;
    const text = editing.text.trim();
    // A message with an attachment may lose its caption.
    if (text !== editing.old && (text || editing.upload)) onEdit?.(editing.id, text);
    editing = null;
    messageInput?.focus();
  };
  // Replies and reactions: stored chats we may write in. picking: the
  // message whose emoji picker is open (focus moves into it and back).
  const canReply = $derived((stored || kind === "text") && !!onSend);
  const canReact = $derived(canReply && !!onReact && myId !== null);
  let picking = $state<number | null>(null);
  const inLine = (id: number | null, selector: string) =>
    document.querySelector<HTMLElement>(`[data-message="${id}"] ${selector}`);
  const openPicker = async (id: number) => {
    picking = id;
    await tick();
    inLine(id, ".actions .emoji")?.focus();
  };
  const closePicker = async () => {
    const id = picking;
    picking = null;
    await tick();
    inLine(id, '[aria-label="Add a reaction"]')?.focus();
  };
  const startConfirm = async (id: number) => {
    confirming = id;
    await tick();
    cancelButton?.focus();
  };
  const endConfirm = () => {
    confirming = null;
    messageInput?.focus();
  };
  // Up in an empty message box edits our last message.
  const editLast = () => {
    const last = messages.findLast((m) => "source" in m && m.source && mayEdit?.(m));
    if (!last || !onEdit) return false;
    void startEdit(last);
    return true;
  };

  // Stored messages carry their id and the message they answer.
  const idOf = (m: ChatMessage | DisplayMessage) => ("source" in m ? m.source?.id : undefined);
  const replyOf = (m: ChatMessage | DisplayMessage) => ("reply" in m ? m.reply : undefined);
  // Mentions as notifications see them: in the written text, not the
  // Markdown result (Mumble messages have no source: their HTML).
  const pingText = (m: ChatMessage | DisplayMessage) => ("source" in m && m.source ? escapeHtml(m.source.text) : m.text);
  // Replies: the message we answer, and jumping to an original.
  let replying = $state<{ id: number; author: string } | null>(null);
  let flashed = $state<number | null>(null);
  const startReply = (m: ChatMessage | DisplayMessage) => {
    if (!("source" in m) || !m.source) return;
    replying = { id: m.source.id, author: m.actorName ?? "" };
    messageInput?.focus();
  };
  const jumpTo = (id: number) => {
    const line = list?.querySelector(`[data-message="${id}"]`);
    if (!line) return; // not loaded
    line.scrollIntoView({ block: "center" });
    flashed = id;
    setTimeout(() => (flashed = flashed === id ? null : flashed), 1500);
  };

  let searchText = $state("");
  // Jump to a search result: page back until it is loaded (a few pages at
  // most), then scroll there.
  const MAX_JUMP_PAGES = 10;
  let jumpPages = 0;
  $effect(() => {
    void jumpTarget;
    jumpPages = 0;
  });
  $effect(() => {
    const id = jumpTarget;
    if (id === null) return;
    if (messages.some((m) => idOf(m) === id)) {
      void tick().then(() => {
        jumpTo(id);
        onJumped?.();
      });
    } else if (more && onLoadOlder && !loadingOlder && jumpPages < MAX_JUMP_PAGES) {
      jumpPages++;
      onLoadOlder();
    }
  });
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
    {#if onMenu}
      <button class="menu" onclick={onMenu} aria-label="Channels" aria-expanded={menuOpen}>
        <svg width="22" height="22" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M4 6h16M4 12h16M4 18h16" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
        </svg>
      </button>
    {/if}
    {#if kind === "dm"}
      <span class="at" aria-hidden="true">@</span>
    {:else}
      <Icon name={kind === "text" ? "hash" : "speaker"} size={22} />
    {/if}
    <h1>{title}</h1>
    {#if topic}
      <span class="divider-v" aria-hidden="true"></span>
      <span class="topic ellipsis" title={topic}>{topic}</span>
    {/if}
    <div class="tools">
      {#if onSearch}
        <form
          class="search"
          role="search"
          onsubmit={(e) => {
            e.preventDefault();
            if (searchText.trim()) onSearch(searchText.trim());
          }}
        >
          <input bind:value={searchText} placeholder="Search" aria-label="Search messages" />
        </form>
      {/if}
      {#if onPins}
        <button class:on={pinsOpen} onclick={onPins} aria-pressed={pinsOpen} title="Pinned messages" aria-label="Pinned messages">
          <Icon name="pin" size={20} />
        </button>
      {/if}
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

  <!-- Clicks on "#name" links open the channel (see markdown). -->
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div
    class="scroller"
    bind:this={list}
    onscroll={trackBottom}
    onclick={(e) => {
      const link = (e.target as Element).closest<HTMLElement>("a[data-channel]");
      if (!link || !isChannelLink(link)) return;
      e.preventDefault();
      onOpenChannel?.(Number(link.dataset.channel));
    }}
  >
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
        {:else if stored}
          This is the start of the {title} voice channel's chat.
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
            <li class="line" class:first={i === 0} class:mentioned={pingsMe(pingText(m), group.actorName, replyOf(m)?.author, me)}
              class:flash={idOf(m) !== undefined && idOf(m) === flashed}
              data-message={idOf(m)}>
              {#if i === 0}
                <div class="gutter">
                  {#if group.actorName}
                    <Avatar name={group.actorName} size={40} registered={"verified" in m && m.verified === true} />
                  {:else}
                    <span class="system-avatar"></span>
                  {/if}
                </div>
                <div class="body">
                  <div class="meta">
                    <span class="author">{group.actorName ?? "Server"}</span>
                    {#if "verified" in m && m.verified}
                      <span class="verified" title="Registered on the server">✓<span class="sr-only">registered</span></span>
                    {/if}
                    <time datetime={new Date(m.at).toISOString()}>{formatTimestamp(m.at, now)}</time>
                  </div>
                  {@render body(m)}
                </div>
              {:else}
                <div class="gutter">
                  <time class="hover-time" datetime={new Date(m.at).toISOString()}>{formatTime(m.at)}</time>
                </div>
                <div class="body">
                  {@render body(m)}
                </div>
              {/if}
              {@render actions(m)}
            </li>
          {/each}
        {/if}
      {/each}
    </ol>
  </div>

  {#if onHoldTalk}
    <button
      class="hold"
      class:held={talking}
      onpointerdown={(e) => {
        if (e.button !== 0) return;
        e.currentTarget.setPointerCapture(e.pointerId);
        held = true;
        onHoldTalk(true);
      }}
      onpointerup={release}
      onpointercancel={release}
      onlostpointercapture={release}
      oncontextmenu={(e) => e.preventDefault()}
    >
      {talking ? "Talking…" : "Hold to talk"}
    </button>
  {/if}
  <form class="send" onsubmit={submit}>
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    {#if pending}
      <div class="pending">
        {#if pending.preview}
          <img src={pending.preview} alt="" />
        {/if}
        <span class="pending-name ellipsis">{pending.file.name}</span>
        <span class="file-size">{formatSize(pending.file.size)}</span>
        <button type="button" class="pending-cancel" onclick={cancelPending} aria-label="Remove attachment">×</button>
      </div>
    {/if}
    {#if replying}
      <div class="replying">
        <span class="ellipsis">Replying to <strong>{replying.author}</strong></span>
        <button type="button" aria-label="Cancel reply" onclick={() => (replying = null)}>×</button>
      </div>
    {/if}
    <div class="composer" class:disabled={!onSend}>
      <button
        type="button"
        class="attach"
        title={onUpload ? "Upload a file" : "Uploads work in text channels and stored voice channel chats"}
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
      {#if suggestions.length}
        <ul class="suggestions" id={listId} role="listbox" aria-label="People to mention">
          {#each suggestions as name, i (name)}
            <!-- The keyboard works through the message box (a combobox). -->
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <li
              id="{listId}-{i}"
              role="option"
              aria-selected={i === active}
              class:selected={i === active}
              onmousedown={(e) => e.preventDefault()}
              onclick={() => pick(name)}
            >
              <Avatar {name} size={20} />
              <span class="ellipsis">{name}</span>
            </li>
          {/each}
        </ul>
      {/if}
      <textarea
        rows="1"
        bind:this={messageInput}
        use:autosize={draft}
        enterkeyhint="send"
        onpaste={onPaste}
        oninput={(e) => {
          updateCaret();
          if (e.currentTarget.value.trim()) onTyping?.();
        }}
        onclick={updateCaret}
        onkeyup={(e) => {
          // Up and Down move in the open list; other keys may move the caret.
          if (!(suggestions.length && (e.key === "ArrowUp" || e.key === "ArrowDown"))) updateCaret();
        }}
        onkeydown={(e) => {
          // Enter picks a word in an input method (229: Safari, after
          // compositionend).
          if (e.isComposing || e.keyCode === 229) return;
          if (suggestions.length && !e.shiftKey && onSuggestionKey(e)) return;
          if (e.key === "ArrowUp" && draft === "" && !pending && editLast()) {
            e.preventDefault();
            return;
          }
          // Enter sends; Shift+Enter makes a new line.
          if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            e.currentTarget.form?.requestSubmit();
            return;
          }
          if (e.key === "Escape" && replying) replying = null;
          else if (e.key === "Escape" && pending) cancelPending();
        }}
        role="combobox"
        aria-autocomplete="list"
        aria-expanded={suggestions.length > 0}
        aria-controls={suggestions.length ? listId : undefined}
        aria-activedescendant={suggestions.length ? `${listId}-${active}` : undefined}
        bind:value={draft}
        placeholder={pending ? "Add a caption (optional), Enter to send" : onSend ? `Message ${target}` : `${title} left the server`}
        disabled={!onSend}
        aria-label="Message"
      ></textarea>
    </div>
    <p class="typing ellipsis" aria-live="polite">{typing}</p>
  </form>
</main>

{#if lightbox}
  <Lightbox url={lightbox.url} name={lightbox.name} onClose={() => (lightbox = null)} />
{/if}

{#snippet body(m: ChatMessage | DisplayMessage)}
  {#if "reply" in m && m.reply}
    {@const quote = m.reply}
    <button type="button" class="quote" onclick={() => jumpTo(quote.id)}>
      {#if quote.author}<strong>{quote.author}</strong> <span class="ellipsis">{quote.text}</span>
      {:else}<em>The original message was deleted.</em>{/if}
    </button>
  {/if}
  {#if "source" in m && m.source && editing?.id === m.source.id}
    <form class="edit" onsubmit={(e) => (e.preventDefault(), saveEdit())}>
      <textarea
        rows="1"
        bind:this={editInput}
        bind:value={editing.text}
        use:autosize={editing.text}
        aria-label="Edit message"
        onkeydown={(e) => {
          if (e.isComposing || e.keyCode === 229) return;
          if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            saveEdit();
          } else if (e.key === "Escape") {
            editing = null;
            messageInput?.focus();
          }
        }}
      ></textarea>
      <span class="hint">Escape to cancel, Enter to save, Shift+Enter for a new line</span>
    </form>
  {:else if m.text}
    <div class="text">
      {@html safe(highlightMentions(m.text, me))}
      {#if "edited" in m && m.edited}<span class="edited" title={new Date(m.edited).toLocaleString()}>(edited)</span>{/if}
      <!-- Styled as the edited mark. -->
      {#if "pinned" in m && m.pinned}<span class="edited">(pinned)</span>{/if}
    </div>
  {/if}
  {@render attachment(m)}
  {#if "source" in m && m.source && previewOf?.(m.source.id)}
    {@const p = previewOf(m.source.id)!}
    <a class="preview" href={p.url} target="_blank" rel="noopener noreferrer">
      <span class="preview-text">
        {#if p.site}<span class="preview-site ellipsis">{p.site}</span>{/if}
        <span class="preview-title">{p.title}</span>
        {#if p.description}<span class="preview-description">{p.description}</span>{/if}
      </span>
      <!-- The server may have let the image go (its cache): then none. -->
      {#if p.image}<img src={httpBase + p.image} alt="" loading="lazy" onerror={(e) => e.currentTarget.remove()} />{/if}
    </a>
  {/if}
  {#if "reactions" in m && m.reactions?.length && "source" in m && m.source}
    {@const id = m.source.id}
    <div class="reactions">
      {#each m.reactions as r (r.emoji)}
        {@const mine = reactedBy(r, myId)}
        <button
          type="button"
          class="reaction"
          class:mine
          aria-pressed={mine}
          aria-label="{r.emoji} {r.users.length}, from {r.users.join(', ')}"
          title={r.users.join(", ")}
          disabled={!canReact}
          onclick={() => onReact?.(id, r.emoji, !mine)}><span class="emoji">{r.emoji}</span>{r.users.length}</button
        >
      {/each}
    </div>
  {/if}
{/snippet}

{#snippet actions(m: ChatMessage | DisplayMessage)}
  {#if "source" in m && m.source && editing?.id !== m.source.id}
    {@const id = m.source.id}
    {@const canEdit = mayEdit?.(m) && onEdit}
    {@const canDelete = mayDelete?.(m) && onDelete}
    {@const canPin = mayPin && onPin}
    {@const pinned = "pinned" in m && !!m.pinned}
    {#if canEdit || canDelete || canReply || canReact || canPin}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="actions"
        class:confirming={confirming === id || picking === id}
        onkeydown={(e) => {
          if (e.key === "Escape" && confirming === id) endConfirm();
          if (e.key === "Escape" && picking === id) void closePicker();
        }}
      >
        {#if picking === id}
          {#each COMMON_EMOJI as emoji (emoji)}
            {@const mine = !!("reactions" in m && m.reactions?.some((r) => r.emoji === emoji && reactedBy(r, myId)))}
            <button
              type="button"
              class="emoji"
              class:mine
              aria-label="React with {emoji}"
              onclick={() => (onReact?.(id, emoji, !mine), closePicker())}>{emoji}</button
            >
          {/each}
          <button type="button" aria-label="Close" onclick={closePicker}>×</button>
        {:else if confirming === id}
          <span>Delete this message?</span>
          <button type="button" class="danger" onclick={() => (onDelete?.(id), endConfirm())}>Delete</button>
          <button type="button" bind:this={cancelButton} onclick={endConfirm}>Cancel</button>
        {:else}
          {#if canReact}
            <button type="button" aria-label="Add a reaction" title="React" onclick={() => openPicker(id)}>React</button>
          {/if}
          {#if canReply}
            <button type="button" aria-label="Reply" title="Reply" onclick={() => startReply(m)}>Reply</button>
          {/if}
          {#if canPin}
            <button type="button" aria-label={pinned ? "Unpin message" : "Pin message"} title={pinned ? "Unpin" : "Pin"} onclick={() => onPin?.(id, !pinned)}
              >{pinned ? "Unpin" : "Pin"}</button
            >
          {/if}
          {#if canEdit}
            <button type="button" aria-label="Edit message" title="Edit" onclick={() => startEdit(m)}>Edit</button>
          {/if}
          {#if canDelete}
            <button type="button" aria-label="Delete message" title="Delete" onclick={() => startConfirm(id)}>Delete</button>
          {/if}
        {/if}
      </div>
    {/if}
  {/if}
{/snippet}

{#snippet attachment(m: ChatMessage | DisplayMessage)}
  {#if "upload" in m && m.upload}
    {#if m.upload.type.startsWith("image/")}
      {@const upload = m.upload}
      <a
        class="image"
        href={upload.url}
        target="_blank"
        rel="noopener noreferrer"
        aria-label="View {upload.name}"
        onclick={(e) => {
          if (!opensLightbox(e)) return;
          e.preventDefault();
          lightbox = { url: upload.url, name: upload.name };
        }}
      >
        <img src={upload.url} alt={upload.name} loading="lazy" onload={keepBottom} />
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
  .search {
    display: flex;
    align-items: center;
  }
  .search input {
    width: 160px;
    height: 28px;
    min-height: 0;
    padding: 4px 8px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
    font-size: 13px;
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
  .divider-v {
    width: 1px;
    height: 24px;
    margin: 0 4px;
    background: var(--border-subtle);
  }
  .topic {
    color: var(--text-muted);
    font-size: 14px;
  }
  .menu {
    display: inline-flex;
    margin-right: 4px;
    padding: 10px;
    color: var(--text-muted);
  }
  .hold {
    margin: 0 16px 8px;
    padding: 14px;
    border-radius: 8px;
    background: var(--bg-input);
    color: var(--text);
    font-weight: 600;
    touch-action: none;
    user-select: none;
  }
  .hold.held {
    background: var(--accent-strong);
    color: white;
  }
  .tools {
    display: flex;
    align-items: center;
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
  .verified {
    color: var(--accent);
    font-size: 13px;
    font-weight: 700;
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
  /* Markdown (see lib/markdown). */
  .text :global(code) {
    padding: 1px 4px;
    border-radius: 4px;
    background: var(--bg-rail);
    font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
    font-size: 0.875em;
  }
  .text :global(pre) {
    margin: 4px 0;
    padding: 8px 10px;
    border-radius: 6px;
    background: var(--bg-rail);
    white-space: pre-wrap;
  }
  .text :global(pre code) {
    padding: 0;
    background: none;
  }
  .text :global(blockquote) {
    margin: 2px 0;
    padding-left: 10px;
    border-left: 4px solid var(--border-subtle);
  }
  .send {
    flex-shrink: 0;
    padding: 0 16px;
  }
  /* Always there (also empty): the layout does not jump. */
  .typing {
    height: 24px;
    margin: 0;
    font-size: 13px;
    line-height: 24px;
    color: var(--text-muted);
  }
  .error {
    margin: 0 0 8px;
    color: var(--danger);
    font-size: 14px;
  }
  .composer {
    position: relative;
    display: flex;
    align-items: flex-end;
    gap: 4px;
    padding: 0 12px 0 4px;
    border-radius: 8px;
    background: var(--bg-input);
  }
  .chat.dragging .scroller {
    outline: 2px dashed var(--accent);
    outline-offset: -8px;
  }
  .suggestions {
    position: absolute;
    left: 0;
    right: 0;
    bottom: calc(100% + 6px);
    margin: 0;
    padding: 6px;
    list-style: none;
    border-radius: 8px;
    background: var(--bg-rail);
    box-shadow: 0 4px 16px rgb(0 0 0 / 0.35);
    z-index: 5;
  }
  .suggestions li {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    border-radius: 4px;
    cursor: pointer;
    --avatar-cutout: var(--bg-rail);
  }
  .suggestions li.selected,
  .suggestions li:hover {
    background: var(--bg-active);
  }
  .line {
    position: relative;
  }
  .actions {
    position: absolute;
    top: -12px;
    right: 16px;
    display: flex;
    opacity: 0;
    align-items: center;
    gap: 2px;
    padding: 2px;
    border-radius: 6px;
    background: var(--bg-rail);
    box-shadow: 0 2px 8px rgb(0 0 0 / 0.3);
    font-size: 12px;
    z-index: 2;
  }
  .line:hover .actions,
  .line:focus-within .actions,
  .actions.confirming {
    opacity: 1;
  }
  /* Phones: no hover, so the actions always show. */
  @media (hover: none) {
    .actions {
      position: static;
      flex-wrap: wrap;
      opacity: 1;
      box-shadow: none;
      background: none;
      justify-content: flex-end;
    }
  }
  .actions span {
    padding: 0 6px;
  }
  .actions button {
    padding: 4px 8px;
    border-radius: 4px;
    color: var(--text);
  }
  .actions button:hover {
    background: var(--bg-active);
  }
  .actions .danger {
    color: var(--danger);
  }
  .actions .emoji {
    padding: 2px 4px;
    font-size: 18px;
  }
  .actions .emoji.mine {
    background: var(--bg-active);
  }
  .preview {
    display: flex;
    gap: 12px;
    max-width: 480px;
    margin-top: 6px;
    padding: 10px 12px;
    border-left: 4px solid var(--accent);
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    text-decoration: none;
  }
  .preview-text {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .preview-site {
    font-size: 12px;
    color: var(--text-muted);
  }
  .preview-title {
    font-weight: 600;
    color: var(--link);
  }
  .preview:hover .preview-title {
    text-decoration: underline;
  }
  .preview-description {
    display: -webkit-box;
    overflow: hidden;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    font-size: 14px;
    color: var(--text-muted);
  }
  .preview img {
    flex-shrink: 0;
    width: 80px;
    height: 80px;
    border-radius: 4px;
    object-fit: cover;
  }
  .reactions {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin-top: 4px;
  }
  .reaction {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 1px 8px;
    border: 1px solid transparent;
    border-radius: 8px;
    background: var(--bg-input);
    color: var(--text-muted);
    font-size: 14px;
    font-weight: 600;
  }
  .reaction:not(:disabled):hover {
    border-color: var(--border-subtle);
  }
  .reaction.mine {
    border-color: var(--accent);
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    color: var(--text);
  }
  .reaction .emoji {
    font-size: 16px;
  }
  .edited {
    margin-left: 4px;
    font-size: 11px;
    color: var(--text-muted);
  }
  .edit {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .edit textarea {
    max-height: 40vh;
    resize: none;
    padding: 8px 10px;
    border: none;
    border-radius: 6px;
    background: var(--bg-input);
    color: var(--text);
    font: inherit;
  }
  .edit .hint {
    font-size: 11px;
    color: var(--text-muted);
  }
  .replying {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 12px;
    margin: 0 0 4px;
    border-radius: 8px 8px 0 0;
    background: var(--bg-rail);
    font-size: 13px;
    color: var(--text-muted);
  }
  .replying button {
    font-size: 18px;
    line-height: 1;
    color: var(--text-muted);
  }
  .quote {
    display: flex;
    gap: 6px;
    max-width: 100%;
    margin-bottom: 2px;
    padding: 0 0 0 8px;
    border-left: 2px solid var(--border-subtle);
    font-size: 13px;
    color: var(--text-muted);
    text-align: left;
    cursor: pointer;
  }
  .quote strong {
    color: var(--text);
    flex: none;
  }
  .line.flash {
    background: rgb(88 101 242 / 0.2);
    transition: background 0.3s;
  }
  .line.mentioned {
    background: rgb(240 178 50 / 0.1);
    box-shadow: inset 2px 0 0 rgb(240 178 50);
  }
  .text :global(.mention) {
    padding: 0 2px;
    border-radius: 3px;
    background: rgb(240 178 50 / 0.25);
    font-weight: 600;
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
  .pending {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 8px;
    padding: 8px 12px;
    border-radius: 8px;
    background: var(--bg-sidebar);
  }
  .pending img {
    width: 48px;
    height: 48px;
    border-radius: 6px;
    object-fit: cover;
  }
  .pending-name {
    flex: 1;
  }
  .pending-cancel {
    font-size: 20px;
    line-height: 1;
    color: var(--text-muted);
  }
  .pending-cancel:hover {
    color: var(--text);
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
    padding: 0; /* the browser's button padding would push the icon aside */
    margin: 10px 8px 10px 8px;
    place-items: center;
    border-radius: 50%;
    background: var(--text-muted);
    color: var(--bg-input);
  }
  .composer textarea {
    flex: 1;
    min-width: 0;
    max-height: 40vh;
    padding: 11px 0;
    border: none;
    background: none;
    color: inherit;
    font: inherit;
    font-size: 16px;
    line-height: 22px;
    resize: none;
    outline: none;
  }
  input::placeholder,
  .composer textarea::placeholder {
    color: var(--text-faint);
  }
  /* Phones: touch targets of at least 44 px. */
  @media (max-width: 700px) {
    .tools button {
      padding: 11px;
    }
  }
  /* The member list is hidden on narrow screens: so is its toggle. */
  @media (max-width: 900px) and (min-width: 701px) {
    .tools .members-toggle {
      display: none;
    }
  }
</style>
