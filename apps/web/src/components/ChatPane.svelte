<script lang="ts">
  import { autosize } from "../lib/autosize";
  import { asSpoiler, mediaKind } from "../lib/media";
  import { emojiImages } from "../lib/emojiImages.svelte";
  import Emoji from "./Emoji.svelte";
  import EmojiPicker from "./EmojiPicker.svelte";
  import MessageSheet, { type SheetItem } from "./MessageSheet.svelte";
  import { hold } from "../lib/longPress";
  import { htmlToText } from "../lib/htmlText";
  import { hideSpoilers, revealSpoiler } from "../lib/markdown";
  import { loadEmojiData, normalEmoji, quickReactions, searchEmoji, withTone, type EmojiData } from "../lib/emojiPicker";
  import { recentEmoji, skinTone, usedEmoji } from "../lib/emojiPrefs.svelte";
  import { customEmojiFor, customEntries, knownEmoji } from "../lib/customEmoji";
  import { blockedUsers } from "../lib/settings.svelte";
  import AudioPlayer from "./AudioPlayer.svelte";
  import VideoPlayer from "./VideoPlayer.svelte";
  import { opensLightbox } from "../lib/lightbox";
  import { COMMON_EMOJI, reactedBy } from "../lib/reactions";
  import type { LinkPreview } from "../lib/linkPreview";
  import { highlightMentions, pingsMe, replyPing, suggestNames, SUGGESTIONS } from "../lib/mentions";
  import { quoted, typedFilter, typedWord } from "../lib/suggest";
  import { createCombobox } from "../lib/combobox.svelte";
  import { matchPlaces } from "../lib/switcher";
  import type { SearchPlace } from "../lib/search";
  import Suggestions, { type Suggestion } from "./Suggestions.svelte";
  import Lightbox from "./Lightbox.svelte";
  import Toast from "./Toast.svelte";
  import { escapeHtml, type ChatMessage } from "@kallisti/protocol";
  import { formatSize, type DisplayMessage } from "../lib/textChannels";
  import { chatItems, formatDay, formatTime, formatTimestamp, type MessageGroup } from "../lib/chat";
  import { nextScrollTop } from "../lib/scroll";
  import { jumpStep } from "../lib/jump";
  import { tick, untrack, type Snippet } from "svelte";
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
    channelNames = [],
    searchPlaces = [],
    onSearch,
    onPins,
    onOpenChannel,
    linkTo,
    onLink,
    stage,
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
    cannotWrite = null,
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
    // Text channel names, for "#" suggestions; chats for "in:" in the search.
    channelNames?: readonly string[];
    searchPlaces?: readonly SearchPlace[];
    // Searching stored messages; jumpTarget: a message to scroll to once
    // it is loaded (then onJumped).
    onSearch?: (text: string) => void;
    // Shown between the header and the messages (screen shares).
    stage?: Snippet;
    // A "#name" link in a message was clicked.
    onOpenChannel?: (id: number) => void;
    // A stored message's link ("Copy link"); onLink: a click on a link,
    // for links to messages here (see lib/messageLink), else null.
    linkTo?: (id: number) => string;
    onLink?: (href: string) => "opened" | "unavailable" | null;
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
    // Why we cannot write now (a time-out); the message box says it.
    cannotWrite?: string | null;
    // Text channels: attach a file (button, paste or drop), with a caption.
    onUpload?: (file: File, caption: string, reply?: number, noPing?: boolean) => void;
    uploading?: boolean;
    // Missing: the conversation is read-only (the other user left).
    // reply: the stored message it answers (stored chats only).
    onSend?: (text: string, reply?: number, noPing?: boolean) => void;
    onClose?: () => void;
  } = $props();

  let draft = $state("");
  let list: HTMLElement | undefined = $state();
  // "Today at" must turn into "Yesterday at" after midnight.
  let now = $state(Date.now());
  const items = $derived(chatItems(messages, blockedUsers.has));
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
      const switched = target !== shown.target;
      const prepended = !switched && firstAt < shown.firstAt;
      const top = nextScrollTop(before, list.scrollHeight, switched, prepended);
      if (top !== null) list.scrollTop = top;
      shown = { target, firstAt };
      before = null;
      trackBottom();
    });
  });

  let filePicker: HTMLInputElement | undefined = $state();
  // Counted: dragging over child elements fires enter/leave pairs.
  let dragDepth = $state(0);
  const dragging = $derived(dragDepth > 0);
  // Groups of messages from blocked users that were shown anyway.
  let unblocked = $state<ReadonlySet<string>>(new Set());
  const groupKey = (g: MessageGroup) => `${g.actorName} ${g.at}`;
  const unblock = (groups: MessageGroup[]) => (unblocked = new Set([...unblocked, ...groups.map(groupKey)]));
  // Spoiler attachments shown (by URL).
  let shownFiles = $state<ReadonlySet<string>>(new Set());
  // A chosen file waits above the message box until it is sent with Enter
  // (with the typed text as caption) or cancelled.
  let pending = $state<{ file: File; preview: string | null; spoiler: boolean } | null>(null);
  const cancelPending = () => {
    if (pending?.preview) URL.revokeObjectURL(pending.preview);
    pending = null;
  };
  const sendFiles = (files: FileList | null | undefined) => {
    const file = files?.[0];
    if (!file || !onUpload) return;
    cancelPending();
    pending = { file, preview: file.type.startsWith("image/") ? URL.createObjectURL(file) : null, spoiler: false };
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
      const { file, spoiler } = pending;
      onUpload(asSpoiler(file, spoiler), text, replying?.id, replying ? !replying.ping : undefined);
      cancelPending();
      replying = null;
      draft = "";
      return;
    }
    if (!text || !onSend) return;
    onSend(text, replying?.id, replying ? !replying.ping : undefined);
    replying = null;
    draft = "";
  };

  // The image shown large (see Lightbox).
  let lightbox = $state<{ url: string; name: string } | null>(null);
  // Media attachments that cannot play: shown as files.
  let unplayable = $state(new Set<string>());

  // Suggestions while typing "@name", "#channel" or ":emoji" (see
  // lib/suggest, lib/combobox).
  let messageInput: HTMLTextAreaElement | undefined = $state();
  const listId = $props.id();
  // Emoji data: loaded when ":" is first typed (as for the picker); after
  // a failure, again only for the next ":" word.
  let emojiData = $state.raw<EmojiData | null>(null);
  let emojiFailedAt = $state<number | null>(null);
  // Custom server emoji (see Client).
  const customEmoji = customEmojiFor();
  const custom = $derived(customEmoji());
  const box = createCombobox({
    id: listId,
    text: () => draft,
    input: () => messageInput,
    find: typedWord,
    build: (typed): Suggestion[] => {
      if (typed.kind === "@") return suggestNames(people, typed.query, me).map((n) => ({ key: n, label: n, insert: `@${n}`, kind: "user" }));
      if (typed.kind === "#") return suggestNames(channelNames, typed.query, "").map((n) => ({ key: n, label: n, insert: `#${n}`, kind: "channel" }));
      // Custom emoji first; they go in as ":name:".
      return [...searchEmoji(customEntries(custom.list), typed.query), ...searchEmoji(emojiData?.emoji ?? [], typed.query)]
        .slice(0, SUGGESTIONS)
        .map((e) => {
          const emoji = normalEmoji(withTone(e, skinTone.value));
          return { key: e.emoji, label: e.name, insert: emoji, kind: "emoji", emoji };
        });
    },
    setText: (text, at) => setDraft(text, at),
    onPick: (s) => s.kind === "emoji" && s.emoji && usedEmoji(s.emoji),
  });
  $effect(() => {
    const typed = box.typed;
    if (typed?.kind !== ":" || emojiData || emojiFailedAt === typed.start) return;
    loadEmojiData().then(
      (d) => (emojiData = d),
      () => (emojiFailedAt = typed.start),
    );
  });
  const LIST_LABEL = { "@": "People", "#": "Channels", ":": "Emoji", from: "People", in: "Channels" } as const;
  // New text in the message box, with the caret at `at`.
  const setDraft = async (text: string, at: number) => {
    draft = text;
    await tick();
    messageInput?.focus();
    messageInput?.setSelectionRange(at, at);
    box.update();
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
  // The message box's emoji picker, at its button; the choice goes in at
  // the caret.
  let draftEmoji = $state<DOMRect | null>(null);
  const insertEmoji = (emoji: string) => {
    usedEmoji(emoji);
    draftEmoji = null;
    const from = messageInput?.selectionStart ?? draft.length;
    const to = messageInput?.selectionEnd ?? draft.length;
    void setDraft(draft.slice(0, from) + emoji + draft.slice(to), from + emoji.length);
  };
  // The quick reactions; "+" opens all emoji (allEmoji: for that message).
  const quick = $derived(
    quickReactions(
      recentEmoji.value.filter((e) => knownEmoji(e, custom)),
      COMMON_EMOJI,
      8,
    ),
  );
  let allEmoji = $state.raw<{ id: number; anchor: DOMRect } | null>(null);
  // The current message (its reactions change while the picker is open).
  const allEmojiFor = $derived(allEmoji && messages.find((m) => idOf(m) === allEmoji!.id));
  const mineOf = (m: ChatMessage | DisplayMessage, emoji: string) =>
    !!("reactions" in m && m.reactions?.some((r) => r.emoji === emoji && reactedBy(r, myId)));
  const react = (m: ChatMessage | DisplayMessage, id: number, emoji: string) => {
    usedEmoji(emoji);
    onReact?.(id, emoji, !mineOf(m, emoji));
    void closePicker();
  };
  // What we may do with a stored message (the server checks again).
  const allowed = (m: ChatMessage | DisplayMessage) => {
    if (!("source" in m) || !m.source || editing?.id === m.source.id) return null;
    return {
      id: m.source.id,
      canEdit: !!(mayEdit?.(m) && onEdit),
      canDelete: !!(mayDelete?.(m) && onDelete),
      canPin: !!(mayPin && onPin),
      pinned: "pinned" in m && !!m.pinned,
    };
  };

  // Phones: a long press opens the message's actions in a sheet. sheetFor: that
  // message (by id when stored, so it follows updates).
  let sheetFor = $state.raw<{ id?: number; m: ChatMessage | DisplayMessage } | null>(null);
  const sheetMessage = $derived(
    sheetFor && (sheetFor.id === undefined ? sheetFor.m : messages.find((m) => idOf(m) === sheetFor!.id)),
  );
  // A short notice at the bottom (copied, a link that cannot open).
  let notice = $state({ n: 0, text: "" });
  const notify = (text: string) => (notice = { n: notice.n + 1, text });
  const copy = (text: string, done: string) =>
    (navigator.clipboard?.writeText(text) ?? Promise.reject()).then(
      () => notify(done),
      () => notify("Could not copy"),
    );
  const sheetItems = (m: ChatMessage | DisplayMessage): SheetItem[] => {
    const may = allowed(m);
    const done = (f: () => void) => () => ((sheetFor = null), f());
    const text = "source" in m && m.source ? m.source.text : htmlToText(m.text);
    return [
      ...(may && canReply ? [{ label: "Reply", icon: "reply" as const, run: done(() => startReply(m)) }] : []),
      ...(text ? [{ label: "Copy text", icon: "copy" as const, run: done(() => copy(text, "Text copied")) }] : []),
      ...(may && linkTo ? [{ label: "Copy link", icon: "copy" as const, run: done(() => copy(linkTo(may.id), "Link copied")) }] : []),
      ...(may?.canPin
        ? [{ label: may.pinned ? "Unpin message" : "Pin message", icon: "pin" as const, run: done(() => onPin?.(may.id, !may.pinned)) }]
        : []),
      ...(may?.canEdit ? [{ label: "Edit message", icon: "edit" as const, run: done(() => void startEdit(m)) }] : []),
      ...(may?.canDelete
        ? [{ label: "Delete message", icon: "trash" as const, danger: true, confirm: "Delete this message?", run: done(() => onDelete?.(may.id)) }]
        : []),
    ];
  };
  // Not for a message without actions (an unstored image, read-only).
  const openSheet = (m: ChatMessage | DisplayMessage) => {
    const id = idOf(m);
    if (sheetItems(m).length || (canReact && id !== undefined)) sheetFor = { id, m };
  };
  const closePicker = async () => {
    const id = picking;
    picking = null;
    allEmoji = null;
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
  // Mentions as notifications see them: in the written text, not the
  // Markdown result (Mumble messages have no source: their HTML).
  const allOf = (m: ChatMessage | DisplayMessage) => (m as DisplayMessage).mentionAll; // @everyone, @here
  const pingText = (m: ChatMessage | DisplayMessage) => ("source" in m && m.source ? escapeHtml(m.source.text) : m.text);
  // Replies: the message we answer, and jumping to an original.
  // ping: the original's author gets a mention (the "@" switch).
  let replying = $state<{ id: number; author: string; ping: boolean } | null>(null);
  let flashed = $state<number | null>(null);
  const startReply = (m: ChatMessage | DisplayMessage) => {
    if (!("source" in m) || !m.source) return;
    replying = { id: m.source.id, author: m.actorName ?? "", ping: true };
    messageInput?.focus();
  };
  const jumpTo = (id: number) => {
    const line = list?.querySelector(`[data-message="${id}"]`);
    if (!line) return; // not loaded
    line.scrollIntoView({ block: "center" });
    trackBottom(); // now, before an image load could pull it back down
    flashed = id;
    setTimeout(() => (flashed = flashed === id ? null : flashed), 1500);
  };

  let searchText = $state("");
  // Suggestions after "from:" and "in:" (see lib/suggest).
  let searchInput: HTMLInputElement | undefined = $state();
  const searchBox = createCombobox({
    id: `${listId}-search`,
    text: () => searchText,
    input: () => searchInput,
    find: typedFilter,
    build: (typed): Suggestion[] => {
      if (typed.kind === "from")
        return suggestNames(people, typed.query, "").map((n) => ({ key: n, label: n, insert: `from:${quoted(n)}`, kind: "user" }));
      return matchPlaces(searchPlaces, typed.query)
        .slice(0, SUGGESTIONS)
        .map((p) => ({
          key: `${p.kind}${p.id}`,
          label: p.name,
          insert: `in:${p.kind === "text" ? `#${p.name}` : p.kind === "dm" ? `@${quoted(p.name)}` : quoted(p.name)}`,
          kind: p.kind === "text" ? "channel" : p.kind === "dm" ? "user" : "voice",
        }));
    },
    setText: async (text, at) => {
      searchText = text;
      await tick();
      searchInput?.focus();
      searchInput?.setSelectionRange(at, at);
    },
  });
  // Jump to a search result (jumpTarget) or a reply's original (seeking)
  // once it is loaded (see jumpStep).
  let seeking = $state<number | null>(null);
  const wanted = $derived(jumpTarget ?? seeking);
  let jumpPages = 0;
  $effect(() => {
    void wanted;
    jumpPages = 0;
  });
  $effect(() => {
    const id = wanted;
    if (id === null) return;
    const loaded = messages.some((m) => idOf(m) === id);
    const step = jumpStep({ loaded, more: more && !!onLoadOlder, loading: loadingOlder, pages: jumpPages });
    if (step === "load") {
      jumpPages++;
      onLoadOlder?.();
    } else if (step !== "wait") {
      seeking = null;
      onJumped?.();
      if (step !== "jump") return;
      // In hidden messages of a blocked user: shown first.
      const hidden = items.find((i) => i.kind === "blocked" && i.groups.some((g) => g.messages.some((m) => idOf(m) === id)));
      if (hidden?.kind === "blocked") unblock(hidden.groups);
      void tick().then(() => jumpTo(id));
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
          <input
            bind:this={searchInput}
            bind:value={searchText}
            placeholder="Search"
            aria-label="Search messages"
            title="Filters: from:name, in:#channel"
            {...searchBox.aria}
            oninput={searchBox.update}
            onclick={searchBox.update}
            onkeyup={searchBox.onkeyup}
            onkeydown={searchBox.onkeydown}
            onblur={searchBox.onblur}
          />
          {#if searchBox.items.length}
            <Suggestions
              id="{listId}-search"
              label={LIST_LABEL[searchBox.typed!.kind]}
              items={searchBox.items}
              active={searchBox.active}
              onPick={searchBox.pick}
              below
            />
          {/if}
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
  {@render stage?.()}

  <!-- Clicks on "#name" links open the channel, clicks on spoilers show
       them (see markdown). -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="scroller"
    bind:this={list}
    onscroll={trackBottom}
    onkeydown={(e) => {
      if ((e.key === "Enter" || e.key === " ") && revealSpoiler(e.target as Element)) e.preventDefault();
    }}
    onclick={(e) => {
      if (revealSpoiler(e.target as Element)) return e.preventDefault();
      const href = (e.target as Element).closest("a[href]")?.getAttribute("href");
      const opened = href && !e.ctrlKey && !e.metaKey && !e.shiftKey ? onLink?.(href) : null;
      if (opened) {
        e.preventDefault();
        if (opened === "unavailable") notify("You cannot open this message here");
        return;
      }
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
          This is the start of your conversation with {title}.{stored ? "" : " Private messages are not stored."}
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
      {#each items as item (item.kind === "day" ? `day-${item.at}` : (item.kind === "group" ? item.group : item.groups[0]!).messages[0])}
        {#if item.kind === "day"}
          <li class="divider" role="separator"><span>{formatDay(item.at)}</span></li>
        {:else if item.kind === "group"}
          {@render lines(item.group)}
        {:else if item.groups.some((g) => unblocked.has(groupKey(g)))}
          {#each item.groups as group (group.messages[0])}{@render lines(group)}{/each}
        {:else}
          {@const n = item.groups.reduce((a, g) => a + g.messages.length, 0)}
          {@const users = new Set(item.groups.map((g) => g.actorName)).size}
          <li class="blocked-line">
            {n === 1 ? "A message" : `${n} messages`} from {users === 1 ? "a blocked user" : "blocked users"}.
            <button type="button" onclick={() => unblock(item.groups)}>Show</button>
          </li>
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
        <button type="button" class="pending-spoiler" aria-pressed={pending.spoiler} onclick={() => pending && (pending.spoiler = !pending.spoiler)}
          >Spoiler</button
        >
        <button type="button" class="pending-cancel" onclick={cancelPending} aria-label="Remove attachment">×</button>
      </div>
    {/if}
    {#if replying}
      <div class="replying">
        <span class="ellipsis">Replying to <strong>{replying.author}</strong></span>
        <button
          type="button"
          class="ping"
          class:off={!replying.ping}
          aria-pressed={replying.ping}
          title={replying.ping ? "The author gets a mention" : "The author gets no mention"}
          onclick={() => replying && (replying.ping = !replying.ping)}>@ {replying.ping ? "On" : "Off"}</button
        >
        <button type="button" aria-label="Cancel reply" onclick={() => (replying = null)}>×</button>
      </div>
    {/if}
    <div class="composer" class:disabled={!onSend || cannotWrite}>
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
      {#if box.items.length}
        <Suggestions id={listId} label={LIST_LABEL[box.typed!.kind]} items={box.items} active={box.active} onPick={box.pick} />
      {/if}
      <textarea
        rows="1"
        bind:this={messageInput}
        use:autosize={draft}
        enterkeyhint="send"
        onpaste={onPaste}
        oninput={(e) => {
          box.update();
          if (e.currentTarget.value.trim()) onTyping?.();
        }}
        onclick={box.update}
        onkeyup={box.onkeyup}
        onblur={box.onblur}
        onkeydown={(e) => {
          if (box.onkeydown(e)) return;
          // Enter picks a word in an input method (229: Safari, after
          // compositionend).
          if (e.isComposing || e.keyCode === 229) return;
          if (e.key === "ArrowUp" && !e.altKey && draft === "" && !pending && editLast()) {
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
        {...box.aria}
        bind:value={draft}
        placeholder={cannotWrite ?? (pending ? "Add a caption (optional), Enter to send" : onSend ? `Message ${target}` : `${title} left the server`)}
        disabled={!onSend || !!cannotWrite}
        aria-label="Message"
      ></textarea>
      {#if onSend}
        <button
          type="button"
          class="emoji-button"
          aria-label="Add an emoji"
          title="Emoji"
          onclick={(e) => (draftEmoji = e.currentTarget.getBoundingClientRect())}><Emoji text="🙂" /></button
        >
      {/if}
    </div>
    <p class="typing ellipsis" aria-live="polite">{typing}</p>
  </form>
</main>

{#if sheetMessage}
  {@const m = sheetMessage}
  {@const id = idOf(m)}
  <MessageSheet
    {quick}
    mine={(emoji) => mineOf(m, emoji)}
    onReact={canReact && id !== undefined ? (emoji) => (react(m, id, emoji), (sheetFor = null)) : undefined}
    items={sheetItems(m)}
    onClose={() => (sheetFor = null)}
  />
{/if}
{#if draftEmoji}
  <EmojiPicker anchor={draftEmoji} onPick={insertEmoji} onClose={() => ((draftEmoji = null), messageInput?.focus())} />
{/if}
{#if allEmoji && allEmojiFor}
  {@const { id, anchor } = allEmoji}
  {@const m = allEmojiFor}
  <EmojiPicker
    {anchor}
    label="Add a reaction"
    selected={(emoji) => mineOf(m, emoji)}
    onPick={(emoji) => react(m, id, emoji)}
    onClose={() => ((allEmoji = null), inLine(id, '[aria-label="More emoji"]')?.focus())}
  />
{/if}
{#if notice.n}
  {#key notice.n}<Toast text={notice.text} />{/key}
{/if}
{#if lightbox}
  <Lightbox url={lightbox.url} name={lightbox.name} onClose={() => (lightbox = null)} />
{/if}

{#snippet lines(group: MessageGroup)}
  {#each group.messages as m, i (m)}
    <li class="line" class:first={i === 0} class:mentioned={pingsMe(pingText(m), group.actorName, replyPing(m as DisplayMessage), me, allOf(m))}
      class:flash={idOf(m) !== undefined && idOf(m) === flashed}
      class:sheet-open={sheetMessage === m}
      use:hold={() => openSheet(m)}
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
{/snippet}

{#snippet body(m: ChatMessage | DisplayMessage)}
  {#if "reply" in m && m.reply}
    {@const quote = m.reply}
    <button type="button" class="quote" onclick={() => quote.author && (seeking = quote.id)}>
      {#if blockedUsers.has(quote.author)}<em>A message from a blocked user.</em>
      {:else if quote.author}<strong>{quote.author}</strong> <span class="ellipsis">{hideSpoilers(quote.text)}</span>
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
      {@html safe(highlightMentions(m.text, me, allOf(m)), emojiImages.ok, custom.urls)}
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
          onclick={() => onReact?.(id, r.emoji, !mine)}><span class="emoji"><Emoji text={r.emoji} /></span>{r.users.length}</button
        >
      {/each}
    </div>
  {/if}
{/snippet}

{#snippet actions(m: ChatMessage | DisplayMessage)}
  {@const may = allowed(m)}
  {#if may}
    {@const { id, canEdit, canDelete, canPin, pinned } = may}
    {#if canEdit || canDelete || canReply || canReact || canPin || linkTo}
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
          {#each quick as emoji (emoji)}
            <button
              type="button"
              class="emoji"
              class:mine={mineOf(m, emoji)}
              aria-label="React with {emoji}"
              onclick={() => react(m, id, emoji)}><Emoji text={emoji} /></button
            >
          {/each}
          <button
            type="button"
            aria-label="More emoji"
            title="More emoji"
            onclick={(e) => (allEmoji = { id, anchor: e.currentTarget.getBoundingClientRect() })}>+</button
          >
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
          {#if linkTo}
            <button type="button" aria-label="Copy link to message" title="Copy link" onclick={() => copy(linkTo(id), "Link copied")}>Link</button>
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
    {#if m.upload.spoiler && !shownFiles.has(m.upload.url)}
      {@const upload = m.upload}
      <button type="button" class="spoiler-file" aria-label="Spoiler, show the attachment" onclick={() => (shownFiles = new Set([...shownFiles, upload.url]))}>
        {#if upload.type.startsWith("image/")}<img src={upload.url} alt="" loading="lazy" onload={keepBottom} />{/if}
        <span>Spoiler</span>
      </button>
    {:else if m.upload.type.startsWith("image/")}
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
    {:else if mediaKind(m.upload) && !unplayable.has(m.upload.url)}
      {@const u = m.upload}
      {@const failed = () => (unplayable = new Set([...unplayable, u.url]))}
      {#if mediaKind(u) === "audio"}
        <AudioPlayer url={u.url} name={u.name} size={u.size} onError={failed} />
      {:else}
        <VideoPlayer url={u.url} name={u.name} size={u.size} onLoad={keepBottom} onError={failed} />
      {/if}
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
    position: relative; /* for the suggestion list */
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
  .text :global(img:not(.emoji)) {
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
  .text :global(.spoiler) {
    border-radius: 4px;
    background: var(--bg-rail);
  }
  .text :global(.spoiler:not(.shown)) {
    color: transparent;
    cursor: pointer;
    user-select: none;
  }
  .text :global(.spoiler:not(.shown) *) {
    color: transparent;
    visibility: hidden;
  }
  .text :global(.spoiler:not(.shown):hover) {
    background: var(--bg-active);
  }
  .blocked-line {
    padding: 4px 16px 4px 72px;
    color: var(--text-muted);
    font-size: 14px;
  }
  .blocked-line button {
    padding: 0;
    border: none;
    background: none;
    color: var(--link);
    font: inherit;
    cursor: pointer;
  }
  .blocked-line button:hover {
    text-decoration: underline;
  }
  .spoiler-file {
    position: relative;
    display: grid;
    min-width: 160px;
    min-height: 80px;
    max-width: 400px;
    margin-top: 4px;
    padding: 0;
    place-items: center;
    border: none;
    border-radius: 8px;
    background: var(--bg-rail);
    overflow: hidden;
    color: inherit;
    cursor: pointer;
  }
  .spoiler-file img {
    grid-area: 1 / 1;
    max-width: 100%;
    max-height: 300px;
    filter: blur(32px);
  }
  .spoiler-file span {
    grid-area: 1 / 1;
    color: #fff;
    z-index: 1;
    padding: 4px 12px;
    border-radius: 16px;
    background: rgb(0 0 0 / 0.6);
    font-size: 14px;
    font-weight: 700;
    text-transform: uppercase;
  }
  .pending-spoiler {
    padding: 2px 8px;
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    background: none;
    color: var(--text-muted);
    font: inherit;
    font-size: 13px;
    cursor: pointer;
  }
  .pending-spoiler[aria-pressed="true"] {
    border-color: transparent;
    background: var(--accent-strong);
    color: #fff;
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
  /* Touch screens: a long press opens the actions (MessageSheet); no
     text selection, "Copy text" is there instead. */
  @media (hover: none) {
    .actions {
      display: none;
    }
    .line {
      -webkit-touch-callout: none;
      -webkit-user-select: none;
      user-select: none;
    }
    .line textarea {
      -webkit-user-select: text;
      user-select: text;
    }
    .line.sheet-open {
      background: var(--bg-active);
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
  .replying .ping {
    margin-left: auto;
    font-size: 13px;
    font-weight: 700;
    color: var(--accent);
  }
  .replying .ping.off {
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
  .emoji-button {
    display: grid;
    padding: 0;
    margin: 9px 0;
    place-items: center;
    border: none;
    background: none;
    cursor: pointer;
    filter: grayscale(1) opacity(0.6);
  }
  .emoji-button:hover,
  .emoji-button:focus-visible {
    filter: none;
  }
  .emoji-button :global(.emoji) {
    width: 24px;
    height: 24px;
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
