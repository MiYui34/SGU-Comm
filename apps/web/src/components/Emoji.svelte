<script lang="ts">
  import { emojiParts } from "../lib/emoji";
  import { emojiImages } from "../lib/emojiImages.svelte";

  // Text with its emoji as Twemoji images (as text when there are none).
  // lazy: load images when they scroll into view (long lists).
  let { text, lazy = false }: { text: string; lazy?: boolean } = $props();
</script>

{#if emojiImages.ok}
  {#each emojiParts(text) as part}
    {#if "emoji" in part}<img class="emoji" draggable="false" alt={part.emoji} src={part.url} loading={lazy ? "lazy" : undefined} />{:else}{part.text}{/if}
  {/each}
{:else}{text}{/if}
