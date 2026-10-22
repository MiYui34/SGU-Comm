<script lang="ts">
  import { untrack } from "svelte";
  import type { TextState } from "../lib/textChannels";
  import { recentLines, timeOf } from "./chat";

  // 文字频道：上面一排频道，中间是消息，下面是输入框。
  let {
    text,
    onSend,
    onOpen,
    onUnread,
  }: {
    text: TextState;
    onSend: (channelId: number, body: string) => void;
    // 这个频道的旧消息还没读，去要一下（每个频道只问一次）。
    onOpen?: (channelId: number) => void;
    // 别的频道来了几条没看的，外面的导航上要挂个数字。
    onUnread?: (total: number) => void;
  } = $props();

  let current = $state<number | null>(null);
  let draft = $state("");
  // 每个频道看到第几条了；新消息按条数算，不用管服务器给的消息 id。
  let seen = $state<Record<number, number>>({});

  const channels = $derived(text.channels);
  const selected = $derived(channels.find((channel) => channel.id === current) ?? channels[0] ?? null);
  const lines = $derived(selected === null ? [] : recentLines(text, selected.id));

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
    const body = draft.trim();
    if (body === "" || selected === null) return;
    onSend(selected.id, body);
    draft = "";
  }

  function onKey(event: KeyboardEvent) {
    // 回车发送，Shift+回车换行。
    if (event.key !== "Enter" || event.shiftKey) return;
    event.preventDefault();
    send();
  }
</script>

<div class="chat">
  <div class="channels">
    {#each channels as channel (channel.id)}
      <button type="button" class="tab" class:on={channel.id === selected?.id} onclick={() => (current = channel.id)}>
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
          <span class="what">{line.text}</span>
          <span class="when">{timeOf(line.at)}</span>
        </div>
      {/each}
    {/if}
  </div>

  <div class="composer">
    <textarea
      bind:value={draft}
      rows="1"
      placeholder={selected === null ? "先选一个频道" : `在 ${selected.name} 里说点什么`}
      disabled={selected === null}
      onkeydown={onKey}
    ></textarea>
    <button type="button" class="send" disabled={draft.trim() === "" || selected === null} onclick={send}>发送</button>
  </div>
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
    grid-template-columns: 112px minmax(0, 1fr) auto;
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

  .what {
    white-space: pre-wrap;
    word-break: break-word;
  }

  .when {
    color: var(--sg-faint);
    font-size: 11.5px;
    font-variant-numeric: tabular-nums;
  }

  .empty {
    margin: auto;
    color: var(--sg-faint);
    font-size: 13px;
  }

  .composer {
    display: flex;
    gap: 8px;
    padding: 10px 12px;
    border-top: 1px solid var(--sg-line);
  }

  .composer textarea {
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

  .composer textarea:focus {
    outline: none;
    border-color: var(--sg-green);
  }

  .send {
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
</style>
