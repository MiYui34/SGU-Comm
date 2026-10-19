<script lang="ts">
  import type { Snippet } from "svelte";
  import { fitMenu, menuStep } from "../lib/menu";

  let {
    x,
    y,
    up = false,
    label,
    anchor,
    onClose,
    children,
  }: {
    x: number;
    y: number;
    // Open above the point (see fitMenu).
    up?: boolean;
    label: string;
    // The button that opened it: a press there is its own (it toggles).
    anchor?: HTMLElement;
    onClose: () => void;
    children: Snippet;
  } = $props();

  let menu: HTMLElement | undefined = $state();
  let pos = $state({ x: 0, y: 0 });

  // Measure after the first render, then place it inside the window.
  $effect(() => {
    if (!menu) return;
    const { width, height } = menu.getBoundingClientRect();
    pos = fitMenu({ x, y }, { width, height }, { width: innerWidth, height: innerHeight }, up);
  });
  // Focus goes back to where it came from when the menu closes. Only once:
  // a re-run (new props while open) would move focus out and close it.
  $effect(() => {
    if (!menu) return;
    const previous = document.activeElement as HTMLElement | null;
    menu.focus();
    return () => previous?.focus();
  });

  // Tab to an element outside closes the menu. Focus going nowhere
  // (relatedTarget null) is not a reason: window blur has its own handler.
  const closeOnFocusOut = (event: FocusEvent) => {
    const next = event.relatedTarget as Node | null;
    if (next && !menu?.contains(next) && !anchor?.contains(next)) onClose();
  };

  // Arrow keys move between the items that can be used (sliders and
  // selects keep theirs).
  const move = (event: KeyboardEvent) => {
    const from = event.target as HTMLElement;
    if (from !== menu && from.tagName !== "BUTTON") return;
    const items = [...(menu?.querySelectorAll<HTMLElement>("button:not(:disabled)") ?? [])];
    const next = menuStep(event.key, items.indexOf(document.activeElement as HTMLElement), items.length);
    if (next === null) return;
    event.preventDefault();
    items[next]!.focus();
  };

  const closeOutside = (event: PointerEvent) => {
    const target = event.target as Node;
    if (menu && !menu.contains(target) && !anchor?.contains(target)) onClose();
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
  onkeydown={move}
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
