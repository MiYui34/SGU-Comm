<script lang="ts">
  import { untrack } from "svelte";
  import type { TextState } from "../lib/textChannels";
  import { COMMON_EMOJI } from "../lib/reactions";
import Lightbox from "../components/Lightbox.svelte";
import { attachmentView, chatSrc, imageBox, lightboxOf, mediaRatio, videoPreload } from "../lib/media";
  import { recentLines, sizeOf, timeOf, type ChatLine } from "./chat";

  // 文字频道：上面一排频道，中间是消息，下面是输入框。
  let {
    text,
    onSend,
    onUpload,
    onOpen,
    onUnread,
    onReact,
    onEdit,
    onPin,
    onDelete,
    onThread,
    onSearch,
    onPins,
    onThreads,
    onChannelChange,
    focusChannel = null,
    thread = null,
    myName = "",
    myId = null,
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
    // 贴/取消一个表情。
    onReact?: (messageId: number, emoji: string, on: boolean) => void;
    // 改自己发过的话。
    onEdit?: (messageId: number, body: string) => void;
    // 删掉一条我自己的消息（服务器还会再查一次权限）。
    onDelete?: (messageId: number) => void;
    // 从一条消息开会话，名字由外面问。
    onThread?: (messageId: number) => void;
    // 搜索框回车。
    onSearch?: (text: string) => void;
    // 打开当前频道的置顶/话题抽屉。
    onPins?: (channelId: number) => void;
    onThreads?: (channelId: number) => void;
    // 换了频道就报给外面，抽屉里要跟着变。
    onChannelChange?: (channelId: number) => void;
    // 外面指定看哪个频道（搜索结果、话题点过来）。
    focusChannel?: number | null;
    // 展开中的话题：它的 id 就是一个聊天 id，当成临时频道列出来。
    thread?: { id: number; name: string } | null;
    // 置顶/取消置顶（要有权限，服务器会拒绝不该做的）。
    onPin?: (messageId: number, on: boolean) => void;
    // 我的名字：和自己发的话对得上，才给「编辑」。
    myName?: string;
    // 我的用户 id：用来知道自己贴过哪些表情。
    myId?: number | null;
    // 没发言权限的频道不给输入框。
    canWrite?: (channelId: number) => boolean;
  } = $props();

  let current = $state<number | null>(null);
  let draft = $state("");

  // 每个频道看到第几条了；新消息按条数算，不用管服务器给的消息 id。
  let seen = $state<Record<number, number>>({});
  let replyTo = $state<ChatLine | null>(null);
  // 点开的大图。
let viewing = $state<{ url: string; name: string; download?: string } | null>(null);
let pending = $state<{ key: string; name: string; size: string } | null>(null);
  let busy = $state(false);
  let error = $state<string | null>(null);
  let picker = $state<HTMLInputElement | null>(null);
  // 正在改的那条，和改到一半的内容。
  let editing = $state<number | null>(null);
  let editingText = $state("");
  // 哪一条下面摊开了表情盘。
  let emojiFor = $state<number | null>(null);

  const channels = $derived(
    thread === null ? text.channels : [...text.channels, { id: thread.id, name: thread.name }],
  );

  // 外面换了频道（点了搜索结果、打开了话题）就跟着切。
  $effect(() => {
    const want = focusChannel;
    if (want !== null && want !== untrack(() => current)) untrack(() => (current = want));
  });
  const selected = $derived(channels.find((channel) => channel.id === current) ?? channels[0] ?? null);

  // 外面的抽屉要知道现在看的是哪个频道。
  $effect(() => {
    const channel = selected;
    if (channel !== null) untrack(() => onChannelChange?.(channel.id));
  });
  const lines = $derived(selected === null ? [] : recentLines(text, selected.id, 60, myId));
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

  function mine(line: ChatLine): boolean {
    return myName !== "" && line.author === myName;
  }

  function startEdit(line: ChatLine) {
    editing = line.id;
    editingText = line.text;
  }

  function saveEdit() {
    if (editing === null) return;
    const body = editingText.trim();
    if (body !== "") onEdit?.(editing, body);
    editing = null;
  }

  function toggle(line: ChatLine, emoji: string) {
    const already = line.reactions.find((reaction) => reaction.emoji === emoji);
    onReact?.(line.id, emoji, !(already?.mine ?? false));
    emojiFor = null;
  }

  function pick(channelId: number) {
    current = channelId;
    replyTo = null;
    pending = null;
    error = null;
    editing = null;
    emojiFor = null;
  }
</script>

<div class="chat">
  {#if onSearch || onPins || onThreads}
    <div class="tools">
      {#if onSearch}
        <input
          class="find"
          type="search"
          placeholder="搜索消息"
          onkeydown={(event) => {
            if (event.key !== "Enter") return;
            const query = event.currentTarget.value.trim();
            if (query !== "") onSearch(query);
          }}
        />
      {/if}
      {#if onPins}
        <button type="button" onclick={() => selected && onPins(selected.id)}>置顶</button>
      {/if}
      {#if onThreads}
        <button type="button" onclick={() => selected && onThreads(selected.id)}>话题</button>
      {/if}
    </div>
  {/if}

  <div class="channels">
    {#each channels as channel (channel.id)}
      <button type="button" class="tab" class:on={channel.id === selected?.id} onclick={() => pick(channel.id)}>
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
        <div class="line" class:pinned={line.pinned}>
          <span class="who">{line.author}</span>
          <div class="body">
            {#if line.pinned}
              <span class="tag">置顶</span>
            {/if}
            {#if line.reply}
              <div class="quote">
                <span>{line.reply.author === "" ? "原来的消息" : line.reply.author}</span>
                {line.reply.text}
              </div>
            {/if}

            {#if editing === line.id}
              <div class="edit">
                <textarea bind:value={editingText} rows="2"></textarea>
                <div class="edit-acts">
                  <button type="button" class="save" onclick={saveEdit}>保存</button>
                  <button type="button" onclick={() => (editing = null)}>取消</button>
                </div>
              </div>
            {:else}
              {#if line.text !== ""}
                <span class="what">
                  {line.text}
                  {#if line.edited}<span class="was">已编辑</span>{/if}
                </span>
              {/if}
              {#if line.upload}
                {@const upload = line.upload}
                {@const view = attachmentView(upload, false)}
                {#if view === "image"}
                  {@const box = imageBox(upload)}
                  <button type="button" class="shot" onclick={() => (viewing = lightboxOf(upload))}>
                    <img src={chatSrc(upload)} alt={upload.name} width={box?.width} height={box?.height} loading="lazy" />
                  </button>
                {:else if view === "audio"}
                  <audio class="sound" controls preload="metadata" src={upload.url}></audio>
                {:else if view === "video"}
                  <!-- svelte-ignore a11y_media_has_caption -->
                  <video
                    class="clip"
                    controls
                    preload={videoPreload(upload.preview)}
                    poster={upload.preview}
                    src={upload.url}
                    style:aspect-ratio={mediaRatio(upload)}
                  ></video>
                {:else}
                  <a class="file" href={upload.url} target="_blank" rel="noreferrer">
                    {upload.name}
                    <span>{upload.size}</span>
                  </a>
                {/if}
              {/if}
              {#if line.reactions.length > 0}
                <div class="reacts">
                  {#each line.reactions as reaction (reaction.emoji)}
                    <button
                      type="button"
                      class="react"
                      class:mine={reaction.mine}
                      onclick={() => onReact?.(line.id, reaction.emoji, !reaction.mine)}
                    >
                      {reaction.emoji} <span>{reaction.count}</span>
                    </button>
                  {/each}
                </div>
              {/if}
            {/if}
          </div>
          <span class="when">{timeOf(line.at)}</span>
          <div class="acts">
            <button type="button" class="reply" onclick={() => (replyTo = line)}>回复</button>
            {#if onReact}
              <button
                type="button"
                class="more"
                onclick={() => (emojiFor = emojiFor === line.id ? null : line.id)}
                aria-label="贴个表情"
              >
                表情
              </button>
            {/if}
            {#if onEdit && mine(line)}
              <button type="button" class="edit-btn" onclick={() => startEdit(line)}>编辑</button>
            {/if}
            {#if onPin}
              <button type="button" class="pin" onclick={() => onPin(line.id, !line.pinned)}>
                {line.pinned ? "取消置顶" : "置顶"}
              </button>
            {/if}
            {#if onThread}
              <button type="button" class="thread" onclick={() => onThread(line.id)}>话题</button>
            {/if}
            {#if onDelete && mine(line)}
              <button type="button" class="del" onclick={() => onDelete(line.id)}>删除</button>
            {/if}
          </div>
        </div>
        {#if emojiFor === line.id}
          <div class="palette">
            {#each COMMON_EMOJI as emoji (emoji)}
              <button type="button" onclick={() => toggle(line, emoji)}>{emoji}</button>
            {/each}
          </div>
        {/if}
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

{#if viewing}
  <Lightbox url={viewing.url} name={viewing.name} download={viewing.download} onClose={() => (viewing = null)} />
{/if}

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

  .tools {
    display: flex;
    gap: 6px;
    padding: 8px 10px;
    border-bottom: 1px solid var(--sg-line);
    background: var(--sg-card);
  }

  .find {
    flex: 1;
    min-width: 0;
    padding: 5px 10px;
    font: inherit;
    font-size: 12.5px;
    color: var(--sg-text);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 7px;
  }

  .find:focus {
    outline: none;
    border-color: var(--sg-green);
  }

  .tools button {
    padding: 4px 10px;
    font: inherit;
    font-size: 12.5px;
    color: var(--sg-muted);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 7px;
    cursor: pointer;
  }

  .tools button:hover {
    color: var(--sg-text);
    border-color: var(--sg-green);
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

  .line.pinned .body {
    padding-left: 8px;
    border-left: 2px solid var(--sg-green);
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

  .tag {
    align-self: flex-start;
    padding: 0 6px;
    color: var(--sg-green);
    font-size: 10.5px;
    background: var(--sg-green-soft);
    border-radius: 4px;
  }

  .what {
    white-space: pre-wrap;
    word-break: break-word;
  }

  .was {
    margin-left: 6px;
    color: var(--sg-faint);
    font-size: 11px;
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

  .shot {
    padding: 0;
    margin-top: 6px;
    background: none;
    border: 1px solid var(--sg-line);
    border-radius: 8px;
    overflow: hidden;
    cursor: zoom-in;
    line-height: 0;
  }

  .shot img {
    display: block;
    max-width: 100%;
    height: auto;
  }

  .sound {
    display: block;
    margin-top: 6px;
    width: min(320px, 100%);
  }

  video.clip {
    display: block;
    margin-top: 6px;
    width: min(360px, 100%);
    border-radius: 8px;
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

  .reacts {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .react {
    display: inline-flex;
    gap: 4px;
    align-items: baseline;
    padding: 1px 7px;
    font: inherit;
    font-size: 12px;
    color: var(--sg-muted);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 999px;
    cursor: pointer;
  }

  .react.mine {
    color: var(--sg-green);
    background: var(--sg-green-soft);
    border-color: var(--sg-green);
  }

  .react span {
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }

  .acts {
    display: flex;
    gap: 8px;
    align-items: baseline;
    opacity: 0;
  }

  .line:hover .acts,
  .acts:focus-within {
    opacity: 1;
  }

  .acts button {
    padding: 0;
    color: var(--sg-faint);
    font: inherit;
    font-size: 11.5px;
    background: none;
    border: 0;
    cursor: pointer;
  }

  .acts button:hover {
    color: var(--sg-green);
  }

  .when {
    color: var(--sg-faint);
    font-size: 11.5px;
    font-variant-numeric: tabular-nums;
  }

  .palette {
    display: flex;
    flex-wrap: wrap;
    gap: 2px;
    margin-left: 122px;
    padding: 4px;
    align-self: flex-start;
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 8px;
  }

  .palette button {
    padding: 2px 5px;
    font-size: 15px;
    background: none;
    border: 0;
    border-radius: 5px;
    cursor: pointer;
  }

  .palette button:hover {
    background: var(--sg-green-soft);
  }

  .edit {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .edit textarea {
    padding: 6px 9px;
    font: inherit;
    font-size: 13px;
    color: var(--sg-text);
    background: var(--sg-bg);
    border: 1px solid var(--sg-green);
    border-radius: 8px;
    resize: vertical;
  }

  .edit-acts {
    display: flex;
    gap: 8px;
  }

  .edit-acts button {
    padding: 3px 10px;
    font: inherit;
    font-size: 12px;
    color: var(--sg-muted);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 6px;
    cursor: pointer;
  }

  .edit-acts .save {
    color: var(--sg-card);
    background: var(--sg-green);
    border-color: var(--sg-green);
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
