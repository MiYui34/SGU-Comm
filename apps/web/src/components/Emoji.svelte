<script lang="ts">
  import { emojiParts } from "../lib/emoji";
  import { emojiImages } from "../lib/emojiImages.svelte";
  import { customEmojiFor, customImageOf, refsAsNames } from "../lib/customEmoji";

  // Text with its emoji as Twemoji images (as text when there are none).
  // A custom emoji (":name:") of this server, or a personal one
  // ("<:name:id>", ":name:" once removed), shows its image.
  // lazy: load images when they scroll into view (long lists).
  let { text, lazy = false }: { text: string; lazy?: boolean } = $props();
  const custom = customEmojiFor();
  const image = $derived(customImageOf(text, custom()));
  const shown = $derived(refsAsNames(text));
</script>

{#if image}<img class="emoji custom" draggable="false" alt=":{image.name}:" title=":{image.name}:" src={image.url} loading={lazy ? "lazy" : undefined} />
{:else if emojiImages.ok}
  {#each emojiParts(shown) as part}
    {#if "emoji" in part}<img class="emoji" draggable="false" alt={part.emoji} src={part.url} loading={lazy ? "lazy" : undefined} />{:else}{part.text}{/if}
  {/each}
{:else}{shown}{/if}
