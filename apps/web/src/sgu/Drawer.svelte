<script lang="ts">
  import type { Snippet } from "svelte";

  // 右侧抽屉：上游那些面板（搜索、置顶、话题、成员、设置……）都从这里出来，
  // 关掉就回到外壳本身。
  let {
    open,
    title,
    onClose,
    children,
    wide = false,
    bare = false,
  }: {
    open: boolean;
    title: string;
    onClose: () => void;
    children: Snippet;
    wide?: boolean;
    // 内容自带标题栏（上游的面板都自带），就不再叠一层。
    bare?: boolean;
  } = $props();

  function onKey(event: KeyboardEvent) {
    if (event.key === "Escape") onClose();
  }
</script>

<svelte:window onkeydown={onKey} />

{#if open}
  <button type="button" class="backdrop" aria-label="关掉" onclick={onClose}></button>
  <aside class="drawer" class:wide aria-label={title}>
    {#if !bare}
      <header>
        <h3>{title}</h3>
        <button type="button" class="close" onclick={onClose} aria-label="关掉">×</button>
      </header>
    {/if}
    <div class="body">
      {@render children()}
    </div>
  </aside>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 60;
    padding: 0;
    background: rgb(20 16 10 / 28%);
    border: 0;
    cursor: default;
  }

  .drawer {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    z-index: 61;
    display: flex;
    flex-direction: column;
    width: min(380px, 92vw);
    background: var(--sg-card);
    border-left: 1px solid var(--sg-line);
    box-shadow: -18px 0 40px rgb(44 38 29 / 12%);
  }

  .drawer.wide {
    width: min(560px, 96vw);
  }

  header {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 14px 16px;
    border-bottom: 1px solid var(--sg-line);
  }

  h3 {
    margin: 0;
    font-size: 14px;
    font-weight: 650;
    letter-spacing: -0.01em;
  }

  .close {
    margin-left: auto;
    width: 26px;
    height: 26px;
    font-size: 17px;
    line-height: 1;
    color: var(--sg-muted);
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    cursor: pointer;
  }

  .close:hover {
    color: var(--sg-text);
    border-color: var(--sg-line);
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 12px 14px;
  }

  /* 上游面板自带一套深色样式，这里给它们一点纸色底，别太跳。
     面板内部的变量仍然走它自己的规则。 */
  .body :global(aside.search),
  .body :global(aside.threads) {
    height: 100%;
    background: transparent;
  }
</style>
