<script lang="ts">
  import { untrack } from "svelte";
  import type { TextState } from "../lib/textChannels";
  import { recentLines, sizeOf, timeOf, type ChatLine } from "./chat";

  // 文字频道：上面一排频道，中间是消息，下面是输入框。
  let {
    text,
    onSend,
    onUpload,
    onOpen,
    onUnread,
    canWrite = () => true,
  }: {
    text: TextState;
    onSend: (channelId: number, body: string, extra?: { reply?: number; upload?: string }) => void;
    // 传了就显示「附件」：把文件交给服务端，拿回一个 key，发送时带上去。
    onUpload?: (file: File) => Promise<string>;
    // 这个频道的旧消息还没读，去要一下（每个频道只问一次）。
    onOpen?: (channelId: number) => void;
    // 别的频道来了几条没看的，外面的导航上要挂个数字。
    onUnread?: (total: number) => void;
    // 没发言权限的频道不给输入框。
    canWrite?: (channelId: number) => boolean;
  } = $props();

  let current = $state<number | null>(null);
  let draft = $state("");
  // 每个频道看到第几条了；新消息按条数算，不用管服务器给的消息 id。
  let seen = $state<Record<number, number>>({});
  let replyTo = $state<ChatLine | null>(null);
  let pending = $state<{ key: string; name: string; size: string } | null>(null);
  let busy = $state(false);
  let error = $state<string | null>(null);
  let picker = $state<HTMLInputElement | null>(null);

  const channels = $derived(text.channels);
  const selected = $derived(channels.find((channel) => channel.id === current) ?? channels[0] ?? null);
  const lines = $derived(selected === null ? [] : recentLines(text, selected.id));
  const writable = $derived(selected !== null && canWrite(selected.id));

  function countOf(channelId: number): number {
    const total = (text.messages[channelId] ?? []).length;
    return Math.max(0, total - (seen[channelId] ?? 0));
  }

  const asked = new Set<number>();
  $effect(() => {
    const channel = selected;
    if (channel === null || text.loaded[channel.id] || asked.has(channel.id)) return;
    asked.add(channel.id);
    onOpen?.(channel.id);
  });

  // 正在看的频道就算读过了；别的频道的条数会留着，直到切过去。
  $effect(() => {
    const channel = selected;
    if (channel === null) return;
    const total = (text.messages[channel.id] ?? []).length;
    untrack(() => {
      if ((seen[channel.id] ?? 0) !== total) seen = { ...seen, [channel.id]: total };
    });
  });

  $effect(() => {
    let total = 0;
    for (const channel of channels) total += countOf(channel.id);
    onUnread?.(total);
  });

  function send() {
    if (selected === null || !writable) return;
    const body = draft.trim();
    if (body === "" && pending === null) return;
    onSend(selected.id, body, {
      ...(replyTo === null ? {} : { reply: replyTo.id }),
      ...(pending === null ? {} : { upload: pending.key }),
    });
    draft = "";
    replyTo = null;
    pending = null;
    error = null;
  }

  function onKey(event: KeyboardEvent) {
    // 回车发送，Shift+回车换行。
    if (event.key !== "Enter" || event.shiftKey) return;
    event.preventDefault();
    send();
  }

  async function picked(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0] ?? null;
    input.value = "";
    if (file === null || onUpload === undefined) return;
    if (text.uploadMax !== undefined && file.size > text.uploadMax) {
      error = `文件不能超过 ${sizeOf(text.uploadMax)}`;
      return;
    }
    busy = true;
    error = null;
    try {
      const key = await onUpload(file);
      pending = { key, name: file.name, size: sizeOf(file.size) };
    } catch {
      error = "上传没成功，再试一次";
    } finally {
      busy = false;
    }
  }
</script>

<div class="chat">
  <div class="channels">
    {#each channels as channel (channel.id)}
      <button
        type="button"
        class="tab"
        class:on={channel.id === selected?.id}
        onclick={() => {
          current = channel.id;
          replyTo = null;
          pending = null;
          error = null;
        }}
      >
        {channel.name}
        {#if countOf(channel.id) > 0}
          <span class="dot">{countOf(channel.id)}</span>
        {/if}
      </button>
    {/each}
  </div>

  <div class="lines">
    {#if lines.length === 0}
      <p class="empty">这里还没有消息</p>
    {:else}
      {#each lines as line (line.id)}
        <div class="line">
          <span class="who">{line.author}</span>
          <div class="body">
            {#if line.reply}
              <div class="quote">
                <span>{line.reply.author === "" ? "原来的消息" : line.reply.author}</span>
                {line.reply.text}
              </div>
            {/if}
            {#if line.text !== ""}
              <span class="what">{line.text}</span>
            {/if}
            {#if line.upload}
              <a class="file" href={line.upload.url} target="_blank" rel="noreferrer">
                {line.upload.name}
                <span>{line.upload.size}</span>
              </a>
            {/if}
          </div>
          <span class="when">{timeOf(line.at)}</span>
          <button type="button" class="reply" onclick={() => (replyTo = line)}>回复</button>
        </div>
      {/each}
    {/if}
  </div>

  {#if writable}
    <div class="composer">
      {#if replyTo !== null || pending !== null || error !== null}
        <div class="above">
          {#if replyTo !== null}
            <div class="chip">
              回复 {replyTo.author}：{replyTo.text}
              <button type="button" onclick={() => (replyTo = null)}>取消</button>
            </div>
          {/if}
          {#if pending !== null}
            <div class="chip">
              附件 {pending.name} {pending.size}
              <button type="button" onclick={() => (pending = null)}>移除</button>
            </div>
          {/if}
          {#if error !== null}
            <p class="error" role="alert">{error}</p>
          {/if}
        </div>
      {/if}
      <div class="row">
        {#if onUpload !== undefined}
          <button type="button" class="clip" disabled={busy} onclick={() => picker?.click()}>附件</button>
          <input bind:this={picker} type="file" onchange={picked} hidden />
        {/if}
        <textarea
          bind:value={draft}
          rows="1"
          placeholder={selected === null ? "先选一个频道" : `在 ${selected.name} 里说点什么`}
          disabled={selected === null}
          onkeydown={onKey}
        ></textarea>
        <button
          type="button"
          class="send"
          disabled={busy || (draft.trim() === "" && pending === null)}
          onclick={send}
        >
          {busy ? "上传中" : "发送"}
        </button>
      </div>
    </div>
  {:else}
    <p class="no-write">这个频道你不能发言</p>
  {/if}
</div>

<style>
  .chat {
    display: grid;
    flex: 1;
    min-height: 0;
    grid-template-rows: auto minmax(0, 1fr) auto;
    background: var(--sg-card);
    border: 1px solid var(--sg-line);
    border-radius: var(--sg-radius);
    overflow: hidden;
  }

  .channels {
    display: flex;
    gap: 4px;
    padding: 8px 10px;
    overflow-x: auto;
    border-bottom: 1px solid var(--sg-line);
    background: var(--sg-panel);
  }

  .tab {
    padding: 5px 10px;
    font: inherit;
    font-size: 13px;
    color: var(--sg-muted);
    background: transparent;
    border: 0;
    border-radius: 6px;
    cursor: pointer;
    white-space: nowrap;
  }

  .tab:hover {
    color: var(--sg-text);
    background: var(--sg-green-soft);
  }

  .tab.on {
    color: var(--sg-card);
    background: var(--sg-green);
  }

  .dot {
    margin-left: 6px;
    padding: 0 5px;
    font-size: 10.5px;
    font-weight: 600;
    color: var(--sg-card);
    background: var(--sg-red, #b8442f);
    border-radius: 999px;
  }

  .lines {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px 16px;
    overflow-y: auto;
  }

  .line {
    display: grid;
    grid-template-columns: 112px minmax(0, 1fr) auto auto;
    gap: 10px;
    align-items: baseline;
    font-size: 13px;
  }

  .who {
    color: var(--sg-green);
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }

  .what {
    white-space: pre-wrap;
    word-break: break-word;
  }

  .quote {
    padding-left: 8px;
    overflow: hidden;
    color: var(--sg-faint);
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
    border-left: 2px solid var(--sg-line);
  }

  .quote span {
    color: var(--sg-muted);
    font-weight: 600;
  }

  .file {
    display: inline-flex;
    gap: 6px;
    align-items: baseline;
    align-self: flex-start;
    padding: 3px 9px;
    color: var(--sg-text);
    font-size: 12.5px;
    text-decoration: none;
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 6px;
  }

  .file span {
    color: var(--sg-faint);
    font-size: 11px;
  }

  .file:hover {
    border-color: var(--sg-green);
  }

  .when {
    color: var(--sg-faint);
    font-size: 11.5px;
    font-variant-numeric: tabular-nums;
  }

  .reply {
    padding: 0;
    color: var(--sg-faint);
    font: inherit;
    font-size: 11.5px;
    background: none;
    border: 0;
    opacity: 0;
    cursor: pointer;
  }

  .line:hover .reply,
  .reply:focus-visible {
    opacity: 1;
  }

  .reply:hover {
    color: var(--sg-green);
  }

  .empty {
    margin: auto;
    color: var(--sg-faint);
    font-size: 13px;
  }

  .composer {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px 12px;
    border-top: 1px solid var(--sg-line);
  }

  .above {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .chip {
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 5px 9px;
    overflow: hidden;
    color: var(--sg-muted);
    font-size: 12px;
    white-space: nowrap;
    text-overflow: ellipsis;
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 6px;
  }

  .chip button {
    margin-left: auto;
    padding: 0;
    color: var(--sg-faint);
    font: inherit;
    font-size: 11.5px;
    background: none;
    border: 0;
    cursor: pointer;
  }

  .chip button:hover {
    color: var(--sg-red);
  }

  .error {
    margin: 0;
    color: var(--sg-red);
    font-size: 12px;
  }

  .row {
    display: flex;
    gap: 8px;
  }

  .clip {
    flex: none;
    padding: 0 12px;
    font: inherit;
    font-size: 12.5px;
    color: var(--sg-muted);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 8px;
    cursor: pointer;
  }

  .clip:hover {
    color: var(--sg-text);
    border-color: var(--sg-green);
  }

  .clip:disabled {
    opacity: 0.55;
    cursor: default;
  }

  .row textarea {
    flex: 1;
    min-height: 34px;
    max-height: 120px;
    padding: 8px 10px;
    font: inherit;
    font-size: 13px;
    color: var(--sg-text);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 8px;
    resize: none;
  }

  .row textarea:focus {
    outline: none;
    border-color: var(--sg-green);
  }

  .send {
    flex: none;
    padding: 0 14px;
    font: inherit;
    font-size: 13px;
    color: var(--sg-card);
    background: var(--sg-green);
    border: 0;
    border-radius: 8px;
    cursor: pointer;
  }

  .send:disabled {
    color: var(--sg-faint);
    background: var(--sg-panel);
    cursor: default;
  }

  .no-write {
    margin: 0;
    padding: 14px;
    color: var(--sg-faint);
    font-size: 12.5px;
    text-align: center;
    border-top: 1px solid var(--sg-line);
  }
</style>
