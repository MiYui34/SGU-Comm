<script lang="ts">
  import { onMount, type Snippet } from "svelte";

  // Phones: a panel from the bottom of the screen (see MessageSheet,
  // ChannelSheet). Closes on a tap outside, a drag down or Escape, and
  // then gives focus back (not to a text field: no keyboard popping
  // up); the caller closes it after an action (onClose), or through
  // dismiss to give focus back too. Tab stays in the sheet.
  let { label, onClose, children }: { label: string; onClose: () => void; children: Snippet } = $props();

  const CLOSE_PX = 80; // dragged down this far, it closes

  let panel: HTMLElement | undefined = $state();
  let drag = $state(0);
  let dragFrom = $state<number | null>(null);
  let opener: HTMLElement | null = null;

  onMount(() => {
    opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    panel?.focus();
  });

  // After the content changed under the focus (a step in the sheet).
  export const focus = () => panel?.focus();

  export const dismiss = () => {
    if (!opener?.matches("input, textarea, [contenteditable]")) opener?.focus({ preventScroll: true });
    onClose();
  };

  const FOCUSABLE = "button:not(:disabled), [href], input, select, textarea, [tabindex]:not([tabindex='-1'])";
  const keepTab = (e: KeyboardEvent) => {
    const items = [...(panel?.querySelectorAll<HTMLElement>(FOCUSABLE) ?? [])];
    const first = items[0];
    const last = items.at(-1);
    if (!first || !last) return e.preventDefault();
    const at = document.activeElement;
    if (e.shiftKey && (at === first || at === panel)) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && at === last) {
      e.preventDefault();
      first.focus();
    }
  };
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="backdrop" data-no-swipe onclick={dismiss}></div>
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  class="sheet"
  role="dialog"
  aria-modal="true"
  aria-label={label}
  tabindex="-1"
  bind:this={panel}
  class:dragging={dragFrom !== null}
  style:transform={drag ? `translateY(${drag}px)` : undefined}
  onkeydown={(e) => {
    if (e.key === "Escape") {
      e.stopPropagation();
      dismiss();
    } else if (e.key === "Tab") keepTab(e);
  }}
  ontouchstart={(e) => (dragFrom = panel?.scrollTop ? null : (e.touches[0]?.clientY ?? null))}
  ontouchmove={(e) => {
    const y = e.touches[0]?.clientY;
    if (dragFrom !== null && y !== undefined) drag = Math.max(0, y - dragFrom);
  }}
  ontouchend={() => {
    if (drag > CLOSE_PX) dismiss();
    drag = 0;
    dragFrom = null;
  }}
>
  <div class="handle" aria-hidden="true"></div>
  {@render children()}
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 30;
    background: rgb(0 0 0 / 0.5);
  }
  .sheet {
    position: fixed;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 31;
    max-height: 80vh;
    padding: 8px calc(12px + var(--safe-right)) calc(12px + var(--safe-bottom)) calc(12px + var(--safe-left));
    border-radius: 12px 12px 0 0;
    background: var(--bg-rail);
    overflow-y: auto;
    overscroll-behavior: contain; /* a drag down is not a page reload */
    outline: none;
    transition: transform 0.15s;
  }
  .sheet.dragging {
    transition: none;
  }
  .handle {
    touch-action: none;
    width: 40px;
    height: 4px;
    margin: 0 auto 12px;
    border-radius: 2px;
    background: var(--text-faint);
  }
  /* Rows of actions (MessageSheet, ChannelSheet); global and low in
     specificity: the sheets' own rules win. */
  :global(.sheet-list) {
    margin: 0;
    padding: 0;
    border-radius: 8px;
    background: var(--bg-input);
    list-style: none;
    overflow: hidden;
  }
  :global(.sheet-item) {
    display: flex;
    gap: 12px;
    align-items: center;
    width: 100%;
    min-height: 48px;
    padding: 0 16px;
    border: none;
    background: none;
    color: inherit;
    font: inherit;
    font-size: 16px;
    text-align: left;
  }
  :global(li + li > .sheet-item) {
    border-top: 1px solid var(--bg-rail);
  }
</style>
