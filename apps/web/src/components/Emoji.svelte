<script lang="ts">
  import { emojiParts } from "../lib/emoji";
  import { emojiImages } from "../lib/emojiImages.svelte";
  import { customEmojiFor, customName } from "../lib/customEmoji";

  // Text with its emoji as Twemoji images (as text when there are none).
  // A custom emoji (":name:") of this server shows its image.
  // lazy: load images when they scroll into view (long lists).
  let { text, lazy = false }: { text: string; lazy?: boolean } = $props();
  const custom = customEmojiFor();
  const customUrl = $derived(custom().urls.get(customName(text) ?? ""));
</script>

{#if customUrl}<img class="emoji custom" draggable="false" alt={text} title={text} src={customUrl} loading={lazy ? "lazy" : undefined} />
{:else if emojiImages.ok}
  {#each emojiParts(text) as part}
    {#if "emoji" in part}<img class="emoji" draggable="false" alt={part.emoji} src={part.url} loading={lazy ? "lazy" : undefined} />{:else}{part.text}{/if}
  {/each}
{:else}{text}{/if}
