<script lang="ts" module>
  // insert: what goes in place of the typed word (see lib/suggest).
  export type Suggestion = { key: string; label: string; insert: string; kind: "user" | "channel" | "voice" | "emoji"; emoji?: string };
</script>

<script lang="ts">
  import Avatar from "./Avatar.svelte";
  import Emoji from "./Emoji.svelte";
  import Icon from "./Icon.svelte";

  // A list over a text field (the field is the combobox: the keyboard works
  // there, see listKey). id: for aria-controls and aria-activedescendant.
  // below: under the field (else above it).
  let {
    id,
    label,
    items,
    active,
    onPick,
    below = false,
  }: { id: string; label: string; items: Suggestion[]; active: number; onPick: (s: Suggestion) => void; below?: boolean } = $props();
</script>

<ul class="suggestions" class:below {id} role="listbox" aria-label={label}>
  {#each items as s, i (s.key)}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <li
      id="{id}-{i}"
      role="option"
      aria-selected={i === active}
      class:selected={i === active}
      onmousedown={(e) => e.preventDefault()}
      onclick={() => onPick(s)}
    >
      {#if s.kind === "user"}<Avatar name={s.label} size={20} />
      {:else if s.kind === "emoji" && s.emoji}<Emoji text={s.emoji} />
      {:else}<Icon name={s.kind === "channel" ? "hash" : "speaker"} size={20} />{/if}
      <span class="ellipsis">{s.label}</span>
    </li>
  {/each}
</ul>

<style>
  .suggestions {
    position: absolute;
    left: 0;
    right: 0;
    bottom: calc(100% + 6px);
    margin: 0;
    padding: 6px;
    list-style: none;
    border-radius: 8px;
    background: var(--bg-rail);
    box-shadow: 0 4px 16px rgb(0 0 0 / 0.35);
    z-index: 5;
  }
  /* Under a narrow field (the search box). */
  .below {
    top: calc(100% + 6px);
    right: auto;
    bottom: auto;
    min-width: 240px;
  }
  li {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    border-radius: 4px;
    cursor: pointer;
    --avatar-cutout: var(--bg-rail);
  }
  li :global(.emoji) {
    width: 20px;
    height: 20px;
  }
  li.selected,
  li:hover {
    background: var(--bg-active);
  }
</style>
