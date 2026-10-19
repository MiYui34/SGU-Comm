<script lang="ts">
  import { banAddress, type Ban, type Registered } from "@kallisti/protocol";
  import { untrack } from "svelte";
  import type { CaptchaLevel } from "../lib/captcha";
  import { EMOJI_TYPES, emojiFormError, emojiNameFromFile, type CustomEmoji } from "../lib/customEmoji";
  import {
    connectionLevel,
    formatOnline,
    formatPercent,
    formatPing,
    POLL_MS,
    worstFirst,
    type Connection,
  } from "../lib/connections";
  import { formatList, MAX_NEW_MINUTES, minutesError, rulesFrom, type MessageRules } from "../lib/messageRules";

  // Server settings for moderators and admins: the ban list, the
  // registered users, the custom emoji, and for admins the guests settings,
  // the message rules and the connections (the server checks every change
  // again).
  let {
    bans,
    registered,
    rights,
    onLoad,
    onSetBans,
    onRename,
    onUnregister,
    emoji,
    onAddEmoji,
    onDeleteEmoji,
    guestsReadOnly = null,
    onGuestsReadOnly,
    captcha = "off",
    onCaptcha,
    connections = null,
    onPollConnections,
    messageRules = null,
    onLoadMessageRules,
    onSaveMessageRules,
    onClose,
  }: {
    bans: readonly Ban[] | null;
    registered: readonly Registered[] | null;
    rights: { bans: boolean; editBans: boolean; users: boolean; emoji: boolean };
    onLoad: (what: "bans" | "users") => void;
    onSetBans: (bans: Ban[]) => void;
    onRename: (userId: number, name: string) => void;
    onUnregister: (userId: number) => void;
    emoji: readonly CustomEmoji[];
    onAddEmoji: (name: string, file: File) => Promise<void>;
    onDeleteEmoji: (name: string) => void;
    // Admins: whether guests may only read (null: not ours to change).
    guestsReadOnly?: boolean | null;
    onGuestsReadOnly?: (on: boolean) => void;
    // Admins (with the guests setting): the browser check for new accounts.
    captcha?: CaptchaLevel;
    onCaptcha?: (level: CaptchaLevel) => void;
    // Admins: every connection (null: not loaded yet), asked for again
    // while the tab is open; without onPollConnections, no tab.
    connections?: readonly Connection[] | null;
    onPollConnections?: () => void;
    // Admins: the message rules (null: not loaded yet); without
    // onLoadMessageRules, no tab.
    messageRules?: MessageRules | null;
    onLoadMessageRules?: () => void;
    onSaveMessageRules?: (rules: MessageRules) => void;
    onClose: () => void;
  } = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  let tab = $state<"bans" | "users" | "emoji" | "guests" | "rules" | "connections">(
    untrack(() => (rights.bans ? "bans" : rights.users ? "users" : rights.emoji ? "emoji" : "guests")),
  );
  let renaming = $state<{ id: number; name: string } | null>(null);
  let confirming = $state<number | null>(null);
  $effect(() => dialog?.showModal());
  $effect(() => {
    if (tab === "bans" || tab === "users") onLoad(tab);
  });
  // Only the tab and the right restart polling: the callback is a new
  // function after every state change, the answer's too.
  const mayPoll = $derived(!!onPollConnections);
  $effect(() => {
    if (tab !== "connections" || !mayPoll) return;
    const poll = () => untrack(() => onPollConnections)?.();
    poll();
    const timer = setInterval(poll, POLL_MS);
    return () => clearInterval(timer);
  });
  const sorted = $derived(connections && worstFirst(connections));

  // The message rules: loaded when the tab opens, the form filled with
  // what the server has (again after a save: it tidies the lists).
  const mayRules = $derived(!!onLoadMessageRules);
  $effect(() => {
    if (tab === "rules" && mayRules) untrack(() => onLoadMessageRules)?.();
  });
  let rulesMinutes = $state("");
  let rulesWords = $state("");
  let rulesDomains = $state("");
  let rulesError = $state<string | null>(null);
  $effect(() => {
    if (!messageRules) return;
    rulesMinutes = String(messageRules.newMinutes);
    rulesWords = formatList(messageRules.words);
    rulesDomains = formatList(messageRules.domains);
  });
  const saveRules = (e: SubmitEvent) => {
    e.preventDefault();
    rulesError = minutesError(rulesMinutes);
    if (!rulesError) onSaveMessageRules?.(rulesFrom(rulesMinutes, rulesWords, rulesDomains));
  };

  // A new emoji: name and image; a message says what is wrong.
  let emojiName = $state("");
  let emojiFile = $state<File | null>(null);
  let emojiError = $state<string | null>(null);
  let adding = $state(false);
  let confirmingEmoji = $state<string | null>(null);
  let fileInput: HTMLInputElement | undefined = $state();
  const addEmoji = async (e: SubmitEvent) => {
    e.preventDefault();
    const name = emojiName.trim();
    emojiError = emojiFormError(name, emojiFile, emoji);
    if (emojiError || !emojiFile) return;
    adding = true;
    try {
      await onAddEmoji(name, emojiFile);
      emojiName = "";
      emojiFile = null;
      if (fileInput) fileInput.value = "";
    } catch (err) {
      emojiError = err instanceof Error ? err.message : String(err);
    } finally {
      adding = false;
    }
  };

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
    {#if rights.emoji}
      <button role="tab" aria-selected={tab === "emoji"} onclick={() => (tab = "emoji")}>Emoji</button>
    {/if}
    {#if guestsReadOnly !== null}
      <button role="tab" aria-selected={tab === "guests"} onclick={() => (tab = "guests")}>Guests</button>
    {/if}
    {#if onLoadMessageRules}
      <button role="tab" aria-selected={tab === "rules"} onclick={() => (tab = "rules")}>Message rules</button>
    {/if}
    {#if onPollConnections}
      <button role="tab" aria-selected={tab === "connections"} onclick={() => (tab = "connections")}>Connections</button>
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
  {:else if tab === "guests"}
    <label class="check">
      <input type="checkbox" checked={guestsReadOnly === true} onchange={(e) => onGuestsReadOnly?.(e.currentTarget.checked)} />
      Guests may only read
    </label>
    <p class="note">
      Guests (without an account, or with a guest account) can still read and talk, but they cannot write, react, upload or send
      direct messages. Use it during a spam wave.
    </p>
    <label class="check">
      Browser check for new accounts and guests
      <select value={captcha} onchange={(e) => onCaptcha?.(e.currentTarget.value as CaptchaLevel)}>
        <option value="off">Off</option>
        <option value="normal">Normal</option>
        <option value="hard">Hard</option>
      </select>
    </label>
    <p class="note">
      Before a web browser creates an account or joins as a guest, it does some work by itself: about 1 to 2 seconds (Normal) or 4
      times as long (Hard). This slows down scripts that make many accounts. No third party is involved. Mumble clients and logins
      do no check.
    </p>
  {:else if tab === "rules"}
    {#if messageRules === null}
      <p class="note">Loading…</p>
    {:else}
      <form class="rules" onsubmit={saveRules}>
        <label>
          Limit new accounts for
          <input class="minutes" bind:value={rulesMinutes} inputmode="numeric" aria-label="Minutes" oninput={() => (rulesError = null)} />
          minutes (0: off)
        </label>
        <p class="note">
          Accounts younger than this, and guests, cannot post links, upload files or mention @everyone or @here, and send one
          message every 5 seconds. Accounts from before this setting existed count as old.
        </p>
        <label for="blocked-words">Blocked words and phrases, one per line</label>
        <textarea id="blocked-words" rows="5" bind:value={rulesWords}></textarea>
        <p class="note">Whole words in any case; accents on Latin letters do not count. No patterns.</p>
        <label for="blocked-domains">Blocked link domains, one per line</label>
        <textarea id="blocked-domains" rows="4" bind:value={rulesDomains}></textarea>
        <p class="note">With their subdomains: example.com also blocks www.example.com.</p>
        {#if rulesError}<p class="error" role="alert">{rulesError}</p>{/if}
        <button type="submit">Save</button>
      </form>
      <p class="note">
        Staff are exempt. The author of a refused message sees why (Mumble clients as "permission denied"); the server log notes
        blocked words and domains.
      </p>
    {/if}
  {:else if tab === "connections"}
    {#if sorted === null}
      <p class="note">Loading…</p>
    {:else if sorted.length === 0}
      <p class="note">Nobody is connected.</p>
    {:else}
      <div class="table">
        <table>
          <thead>
            <tr>
              <th>User</th>
              <th>Voice</th>
              <th>Ping</th>
              <th
                title="Late or lost voice in the last 30 to 40 s. ↑ from the user, as the server counted it; ↓ to the user, as their client reported it (Mumble clients count UDP only). –: too little voice."
              >
                Late/lost ↑ · ↓
              </th>
              <th title="In the same time: voice frames the server dropped for this user (their queue was full) · text messages they lost">
                Dropped
              </th>
              <th>Online</th>
              <th>Client</th>
            </tr>
          </thead>
          <tbody>
            {#each sorted as c (c.session)}
              {@const level = connectionLevel(c)}
              <tr>
                <td><span class="level {level ?? 'unknown'}" title={level ?? "not measured yet"}></span>{c.name}</td>
                <td>{c.transport === "websocket" ? "WebSocket" : c.transport.toUpperCase()}</td>
                <td>{formatPing(c.ping)}</td>
                <td>{formatPercent(c.fromClient)} · {formatPercent(c.fromServer)}</td>
                <td>{c.dropped} · {c.overflows}</td>
                <td>{formatOnline(c.online)}</td>
                <td class="detail">{[c.client, c.os].filter(Boolean).join(" · ")}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      <p class="note">Worst first; updates every few seconds. No addresses: admins see them in Mumble's user information.</p>
    {/if}
  {:else if tab === "emoji"}
    <form class="add-emoji" onsubmit={addEmoji}>
      <input
        bind:this={fileInput}
        type="file"
        accept={EMOJI_TYPES.join(",")}
        aria-label="Emoji image"
        onchange={(e) => {
          emojiFile = e.currentTarget.files?.[0] ?? null;
          if (emojiFile && !emojiName) emojiName = emojiNameFromFile(emojiFile.name);
          emojiError = null;
        }}
      />
      <input bind:value={emojiName} placeholder="name" aria-label="Emoji name" maxlength="32" oninput={() => (emojiError = null)} />
      <button type="submit" disabled={adding}>{adding ? "Adding…" : "Add"}</button>
    </form>
    <p class="note">PNG, GIF, WebP or JPEG, at most 256 KB. Names: 2 to 32 lower case letters, digits or _.</p>
    {#if emojiError}<p class="error" role="alert">{emojiError}</p>{/if}
    {#if emoji.length === 0}
      <p class="note">This server has no custom emoji yet.</p>
    {:else}
      <ul>
        {#each emoji as e (e.name)}
          <li>
            <img class="emoji-image" src={e.url} alt="" />
            <div class="what"><strong>:{e.name}:</strong></div>
            {#if confirmingEmoji === e.name}
              <button type="button" class="danger" onclick={() => (onDeleteEmoji(e.name), (confirmingEmoji = null))}>Delete</button>
              <button type="button" onclick={() => (confirmingEmoji = null)}>Cancel</button>
            {:else}
              <button type="button" onclick={() => (confirmingEmoji = e.name)}>Delete…</button>
            {/if}
          </li>
        {/each}
      </ul>
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
    padding-bottom: 12px;
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
  .check {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .check select {
    padding: 4px 6px;
    border: none;
    border-radius: 4px;
    background: var(--bg-input);
    color: var(--text);
    font: inherit;
  }
  .add-emoji {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
  }
  .add-emoji input:not([type="file"]) {
    flex: 1;
    min-width: 8em;
    padding: 6px 8px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
  }
  .add-emoji input[type="file"] {
    max-width: 100%;
  }
  .add-emoji button {
    padding: 6px 10px;
    border-radius: 4px;
    background: var(--bg-input);
    color: var(--text);
  }
  .error {
    color: var(--danger);
    font-size: 14px;
  }
  .table {
    overflow-x: auto;
  }
  .rules {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .rules .note {
    margin: 0 0 8px;
  }
  .rules textarea,
  .rules .minutes {
    padding: 6px 8px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
  }
  .rules textarea {
    resize: vertical;
  }
  .rules .minutes {
    width: 5em;
  }
  .rules button {
    align-self: flex-start;
    padding: 6px 10px;
    border-radius: 4px;
    background: var(--bg-input);
    color: var(--text);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
    white-space: nowrap;
  }
  th,
  td {
    padding: 6px 8px 6px 0;
    border-bottom: 1px solid var(--border-subtle);
    text-align: left;
  }
  th {
    color: var(--text-muted);
    font-weight: 600;
  }
  th[title] {
    cursor: help;
  }
  .level {
    display: inline-block;
    width: 8px;
    height: 8px;
    margin-right: 6px;
    border-radius: 50%;
    background: var(--text-faint);
  }
  .level.good {
    background: var(--online);
  }
  .level.fair {
    background: var(--idle);
  }
  .level.poor {
    background: var(--dnd);
  }
  .emoji-image {
    width: 32px;
    height: 32px;
    object-fit: contain;
  }
</style>
