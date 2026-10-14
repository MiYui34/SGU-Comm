<script lang="ts">
  import type { Snippet } from "svelte";
  import { fitMenu } from "../lib/menu";

  let {
    x,
    y,
    label,
    onClose,
    children,
  }: { x: number; y: number; label: string; onClose: () => void; children: Snippet } = $props();

  let menu: HTMLElement | undefined = $state();
  let pos = $state({ x: 0, y: 0 });

  // Measure after the first render, then place it inside the window. Focus
  // goes back to where it came from when the menu closes.
  $effect(() => {
    if (!menu) return;
    const { width, height } = menu.getBoundingClientRect();
    pos = fitMenu({ x, y }, { width, height }, { width: innerWidth, height: innerHeight });
    const previous = document.activeElement as HTMLElement | null;
    menu.focus();
    return () => previous?.focus();
  });

  // Tab to an element outside closes the menu. Focus going nowhere
  // (relatedTarget null) is not a reason: window blur has its own handler.
  const closeOnFocusOut = (event: FocusEvent) => {
    const next = event.relatedTarget as Node | null;
    if (next && !menu?.contains(next)) onClose();
  };

  const closeOutside = (event: PointerEvent) => {
    if (menu && !menu.contains(event.target as Node)) onClose();
  };
  const closeOnEscape = (event: KeyboardEvent) => {
    if (event.key === "Escape") onClose();
  };
</script>

<svelte:window
  onpointerdowncapture={closeOutside}
  onkeydown={closeOnEscape}
  onblur={onClose}
  onresize={onClose}
/>

<div
  class="menu"
  role="dialog"
  tabindex="-1"
  onfocusout={closeOnFocusOut}
  aria-label={label}
  bind:this={menu}
  style:left="{pos.x}px"
  style:top="{pos.y}px"
>
  {@render children()}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 100;
    min-width: 220px;
    padding: 6px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg-menu);
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.4);
    outline: none;
  }
</style>
