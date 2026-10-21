<script lang="ts">
  // An image large over the app. Closes with Escape, the button, or a
  // click outside the image; the browser puts focus back. download: the
  // original of a picture shown by its preview (see lib/media lightboxOf).
  let { url, name, download, onClose }: { url: string; name: string; download?: string; onClose: () => void } = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  $effect(() => dialog?.showModal());
  // Not only through the close event: browsers send it late in hidden tabs.
  const close = () => {
    dialog?.close();
    onClose();
  };
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<dialog
  bind:this={dialog}
  aria-label={name}
  onclose={onClose}
  onclick={(e) => {
    if (e.target === e.currentTarget || (e.target as Element).classList.contains("stage")) close();
  }}
>
  <div class="stage">
    <img src={url} alt={name} />
  </div>
  <footer>
    <span class="name ellipsis">{name}</span>
    {#if download}
      <a href={download}>Download original</a>
    {:else}
      <a href={url} target="_blank" rel="noopener noreferrer">Open original</a>
    {/if}
    <button type="button" class="close" onclick={close} aria-label="Close">×</button>
  </footer>
</dialog>

<style>
  dialog {
    width: 100vw;
    max-width: 100vw;
    height: 100vh;
    max-height: 100vh;
    inset: 0;
    margin: 0;
    padding: var(--safe-top) var(--safe-right) var(--safe-bottom) var(--safe-left);
    border: none;
    background: none;
    display: flex;
    flex-direction: column;
  }
  dialog:not([open]) {
    display: none;
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.85);
  }
  .stage {
    flex: 1;
    min-height: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
  }
  img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    border-radius: 4px;
  }
  footer {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 8px 16px 16px;
    color: #ddd;
    font-size: 14px;
  }
  .name {
    flex: 1;
    min-width: 0;
  }
  a {
    color: #ddd;
  }
  a:hover {
    color: white;
  }
  .close {
    font-size: 28px;
    line-height: 1;
    color: #ddd;
  }
  .close:hover {
    color: white;
  }
</style>
