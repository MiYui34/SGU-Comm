<script lang="ts" module>
  import type Icon from "./Icon.svelte";
  import type { ComponentProps } from "svelte";

  // confirm: asked first (in the sheet), for actions that cannot be undone.
  export type SheetItem = {
    label: string;
    icon: ComponentProps<typeof Icon>["name"];
    run: () => void;
    danger?: boolean;
    confirm?: string;
  };
</script>

<script lang="ts">
  import { onMount, tick } from "svelte";
  import Emoji from "./Emoji.svelte";
  import EmojiPicker from "./EmojiPicker.svelte";
  import IconView from "./Icon.svelte";

  // Phones: a message's actions, from a long press. Quick reactions and
  // "+" (all emoji) when reacting is allowed (onReact), then the actions.
  // Closes on a tap outside, a drag down or Escape; the caller closes it
  // after an action (onClose).
  let {
    quick,
    mine,
    onReact,
    items,
    onClose,
  }: {
    quick: string[];
    mine: (emoji: string) => boolean;
    onReact?: (emoji: string) => void;
    items: SheetItem[];
    onClose: () => void;
  } = $props();

  const CLOSE_PX = 80; // dragged down this far, it closes

  let confirming = $state<SheetItem | null>(null);
  let picker = $state<DOMRect | null>(null);
  let panel: HTMLElement | undefined = $state();
  let drag = $state(0);
  let dragFrom = $state<number | null>(null);

  onMount(() => panel?.focus());

  // The confirm step replaces the focused button: focus stays in the sheet.
  const confirm = async (item: SheetItem | null) => {
    confirming = item;
    await tick();
    panel?.focus();
  };
  const run = (item: SheetItem) => {
    if (item.confirm && confirming !== item) void confirm(item);
    else item.run();
  };
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="backdrop" data-no-swipe onclick={onClose}></div>
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  class="sheet"
  role="dialog"
  aria-modal="true"
  aria-label="Message actions"
  tabindex="-1"
  bind:this={panel}
  class:dragging={dragFrom !== null}
  style:transform={drag ? `translateY(${drag}px)` : undefined}
  onkeydown={(e) => {
    if (e.key === "Escape") {
      e.stopPropagation();
      onClose();
    }
  }}
  ontouchstart={(e) => (dragFrom = panel?.scrollTop ? null : (e.touches[0]?.clientY ?? null))}
  ontouchmove={(e) => {
    const y = e.touches[0]?.clientY;
    if (dragFrom !== null && y !== undefined) drag = Math.max(0, y - dragFrom);
  }}
  ontouchend={() => {
    if (drag > CLOSE_PX) onClose();
    drag = 0;
    dragFrom = null;
  }}
>
  <div class="handle" aria-hidden="true"></div>
  {#if confirming}
    <p class="question">{confirming.confirm}</p>
    <button type="button" class="item danger" onclick={() => confirming && run(confirming)}>{confirming.label}</button>
    <button type="button" class="item" onclick={() => confirm(null)}>Cancel</button>
  {:else}
    {#if onReact}
      <div class="quick" role="group" aria-label="Reactions">
        {#each quick as emoji (emoji)}
          <button type="button" class:mine={mine(emoji)} aria-label="React with {emoji}" onclick={() => onReact(emoji)}
            ><Emoji text={emoji} /></button
          >
        {/each}
        <button
          type="button"
          class="more"
          aria-label="More emoji"
          onclick={(e) => (picker = e.currentTarget.getBoundingClientRect())}><IconView name="plus" size={22} /></button
        >
      </div>
    {/if}
    <ul>
      {#each items as item (item.label)}
        <li>
          <button type="button" class="item" class:danger={item.danger} onclick={() => run(item)}>
            <IconView name={item.icon} size={20} />{item.label}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>
{#if picker && onReact}
  <EmojiPicker anchor={picker} label="Add a reaction" selected={mine} onPick={onReact} onClose={() => (picker = null)} />
{/if}

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
  .quick {
    display: flex;
    justify-content: space-between;
    margin-bottom: 12px;
  }
  .quick button {
    display: grid;
    width: 44px;
    height: 44px;
    padding: 0;
    place-items: center;
    border: none;
    border-radius: 50%;
    background: var(--bg-input);
    color: var(--text-muted);
  }
  .quick button.mine {
    background: rgb(88 101 242 / 0.3);
  }
  .quick :global(.emoji) {
    width: 26px;
    height: 26px;
  }
  ul {
    margin: 0;
    padding: 0;
    border-radius: 8px;
    background: var(--bg-input);
    list-style: none;
    overflow: hidden;
  }
  .item {
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
  li + li .item {
    border-top: 1px solid var(--bg-rail);
  }
  .item.danger {
    color: var(--danger);
  }
  .question {
    margin: 0 4px 12px;
    font-size: 16px;
    font-weight: 600;
  }
  .question ~ .item {
    margin-top: 8px;
    border-radius: 8px;
    background: var(--bg-input);
    justify-content: center;
  }
</style>
