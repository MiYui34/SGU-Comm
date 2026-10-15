<script lang="ts">
  import ContextMenu from "./ContextMenu.svelte";

  // Right-click menu of a channel. Items show only when the user may use
  // them (the server checks again).
  let {
    name,
    x,
    y,
    canEdit,
    canMoveUp,
    canMoveDown,
    onEdit,
    onMove,
    onDelete,
    onClose,
  }: {
    name: string;
    x: number;
    y: number;
    canEdit: boolean;
    canMoveUp: boolean;
    canMoveDown: boolean;
    onEdit: () => void;
    onMove: (step: -1 | 1) => void;
    onDelete: () => void;
    onClose: () => void;
  } = $props();

  const act = (fn: () => void) => () => {
    fn();
    onClose();
  };
</script>

<ContextMenu {x} {y} label="Actions for {name}" {onClose}>
  {#if canEdit}
    <button onclick={act(onEdit)}>Edit channel</button>
    {#if canMoveUp}<button onclick={act(() => onMove(-1))}>Move up</button>{/if}
    {#if canMoveDown}<button onclick={act(() => onMove(1))}>Move down</button>{/if}
    <div class="sep"></div>
    <button class="danger" onclick={act(onDelete)}>Delete channel</button>
  {:else}
    <p class="none">You may not change this channel.</p>
  {/if}
</ContextMenu>

<style>
  .sep {
    height: 1px;
    margin: 4px 6px;
    background: var(--border-subtle);
  }
  button {
    display: block;
    width: 100%;
    padding: 8px;
    border-radius: 4px;
    font-size: 14px;
    text-align: left;
    color: var(--text-muted);
  }
  button:hover {
    background: var(--accent-strong);
    color: white;
  }
  .danger {
    color: var(--danger);
  }
  .danger:hover {
    background: var(--danger);
  }
  .none {
    margin: 0;
    padding: 8px;
    font-size: 14px;
    color: var(--text-muted);
  }
</style>
