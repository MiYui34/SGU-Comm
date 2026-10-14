<script lang="ts">
  // A short notice at the bottom, gone after 5 seconds. Re-create (with a
  // {#key} block) to show the next one.
  let { text }: { text: string } = $props();
  let visible = $state(true);
  $effect(() => {
    const timer = setTimeout(() => (visible = false), 5000);
    return () => clearTimeout(timer);
  });
</script>

{#if visible}
  <div class="toast" role="status">{text}</div>
{/if}

<style>
  .toast {
    position: fixed;
    bottom: 96px;
    left: 50%;
    z-index: 200;
    max-width: min(480px, 90vw);
    padding: 10px 16px;
    border-radius: 8px;
    background: var(--bg-menu);
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.4);
    transform: translateX(-50%);
  }
</style>
