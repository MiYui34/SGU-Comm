<script lang="ts">
  import { emojiParts } from "../lib/emoji";
  import { emojiImages } from "../lib/emojiImages.svelte";

  // Text with its emoji as Twemoji images (as text when there are none).
  let { text }: { text: string } = $props();
</script>

{#if emojiImages.ok}
  {#each emojiParts(text) as part}
    {#if "emoji" in part}<img class="emoji" draggable="false" alt={part.emoji} src={part.url} />{:else}{part.text}{/if}
  {/each}
{:else}{text}{/if}
