<script lang="ts">
  import { tick, untrack } from "svelte";

  // 站内的小问询框：要一个名字、一句说明时用这个，不走浏览器的 prompt。
  let {
    title,
    label = "",
    value = "",
    placeholder = "",
    ok = "确定",
    multiline = false,
    onDone,
    onCancel,
  }: {
    title: string;
    label?: string;
    // 打开时的初值。
    value?: string;
    placeholder?: string;
    ok?: string;
    // 说明可能比较长，用多行框。
    multiline?: boolean;
    onDone: (text: string) => void;
    onCancel: () => void;
  } = $props();

  let text = $state(untrack(() => value));
  let field = $state<HTMLInputElement | HTMLTextAreaElement | null>(null);

  $effect(() => {
    void tick().then(() => field?.focus());
  });

  function submit(event: SubmitEvent) {
    event.preventDefault();
    const body = text.trim();
    if (body === "") return;
    onDone(body);
  }
</script>

<button type="button" class="backdrop" aria-label="取消" onclick={onCancel}></button>

<form class="ask" onsubmit={submit}>
  <h3>{title}</h3>
  {#if label}<p class="hint">{label}</p>{/if}
  {#if multiline}
    <textarea bind:this={field} bind:value={text} rows="3" {placeholder} maxlength="400"></textarea>
  {:else}
    <input bind:this={field} bind:value={text} {placeholder} maxlength="60" />
  {/if}
  <div class="acts">
    <button type="button" class="no" onclick={onCancel}>取消</button>
    <button type="submit" class="ok" disabled={text.trim() === ""}>{ok}</button>
  </div>
</form>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 80;
    padding: 0;
    background: rgb(20 16 10 / 26%);
    border: 0;
  }

  .ask {
    position: fixed;
    top: 50%;
    left: 50%;
    z-index: 81;
    display: flex;
    flex-direction: column;
    gap: 10px;
    width: min(360px, 88vw);
    padding: 18px;
    background: var(--sg-card);
    border: 1px solid var(--sg-line);
    border-radius: 12px;
    box-shadow: 0 24px 60px rgb(44 38 29 / 20%);
    transform: translate(-50%, -50%);
  }

  .ask h3 {
    margin: 0;
    font-size: 14px;
    font-weight: 650;
  }

  .hint {
    margin: 0;
    color: var(--sg-muted);
    font-size: 12.5px;
    line-height: 1.6;
  }

  input,
  textarea {
    width: 100%;
    padding: 8px 10px;
    font: inherit;
    font-size: 13px;
    color: var(--sg-text);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
    border-radius: 8px;
    resize: vertical;
  }

  input:focus,
  textarea:focus {
    border-color: var(--sg-green);
    outline: none;
  }

  .acts {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }

  .acts button {
    padding: 6px 14px;
    border-radius: 8px;
    font: inherit;
    font-size: 12.5px;
    cursor: pointer;
  }

  .no {
    color: var(--sg-muted);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line);
  }

  .ok {
    color: #fdfbf6;
    background: var(--sg-green);
    border: 1px solid var(--sg-green);
  }

  .ok:disabled {
    color: var(--sg-faint);
    background: var(--sg-panel);
    border-color: var(--sg-line);
    cursor: default;
  }
</style>
