<script lang="ts">
  import { MOD } from "../lib/switcher";

  // Ctrl+/ (Cmd+/ on a Mac): the keyboard shortcuts.
  let { onClose }: { onClose: () => void } = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  $effect(() => dialog?.showModal());
  const keys: [string, string][] = [
    ["Alt+↑ / Alt+↓", "上一个 / 下一个频道"],
    ["Alt+Shift+↑ / ↓", "跳到有未读的频道"],
    ["↑", "编辑自己上一条消息（输入框是空的）"],
    ["Shift+Enter", "换行"],
    ["Esc", "取消回复或编辑，关掉弹窗"],
    [`${MOD}+/`, "这个列表"],
  ];
</script>

<dialog bind:this={dialog} onclose={onClose} aria-labelledby="shortcuts-title">
  <header>
    <h2 id="shortcuts-title">键盘快捷键</h2>
    <button type="button" class="close" onclick={() => dialog?.close()} aria-label="关闭">×</button>
  </header>
  <dl>
    {#each keys as [key, what] (key)}
      <dt><kbd>{key}</kbd></dt>
      <dd>{what}</dd>
    {/each}
  </dl>
</dialog>

<style>
  dialog {
    width: min(480px, 92vw);
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
  }
  h2 {
    margin: 0;
    font-size: 20px;
  }
  .close {
    border: none;
    background: none;
    color: var(--text-muted);
    font-size: 24px;
    cursor: pointer;
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 10px 16px;
    margin: 16px 0 0;
  }
  dt,
  dd {
    margin: 0;
  }
  dd {
    color: var(--text-muted);
  }
  kbd {
    padding: 2px 6px;
    border-radius: 4px;
    background: var(--bg-rail);
    font-family: inherit;
    font-size: 13px;
    white-space: nowrap;
  }
</style>
