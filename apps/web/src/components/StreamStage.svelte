<script lang="ts">
  // The screen shares being watched, above the chat (as Discord's call
  // area): each with its name, full screen and close.
  let {
    sessions,
    nameOf,
    attach,
    onClose,
  }: {
    sessions: number[];
    nameOf: (session: number) => string;
    attach: (session: number, canvas: HTMLCanvasElement | null) => void;
    onClose: (session: number) => void;
  } = $props();

  const canvasOf = (canvas: HTMLCanvasElement, session: number) => {
    attach(session, canvas);
    return { destroy: () => attach(session, null) };
  };
  const fullScreen = (e: MouseEvent) => {
    const tile = (e.currentTarget as HTMLElement).closest(".tile");
    if (document.fullscreenElement) void document.exitFullscreen();
    else void tile?.requestFullscreen();
  };
</script>

<section class="stage" aria-label="Screen shares">
  {#each sessions as session (session)}
    <div class="tile">
      <canvas use:canvasOf={session} aria-label="{nameOf(session)}'s screen"></canvas>
      <div class="overlay">
        <span class="who"><span class="live">LIVE</span> {nameOf(session)}</span>
        <span class="tools">
          <button onclick={fullScreen} title="Full screen" aria-label="Full screen">⛶</button>
          <button onclick={() => onClose(session)} title="Stop watching" aria-label="Stop watching {nameOf(session)}">×</button>
        </span>
      </div>
    </div>
  {/each}
</section>

<style>
  .stage {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    padding: 8px 16px;
    background: #000;
  }
  .tile {
    position: relative;
    flex: 1 1 360px;
    min-width: 0;
    background: #000;
  }
  canvas {
    display: block;
    width: 100%;
    max-height: 55vh;
    object-fit: contain;
  }
  .tile:fullscreen canvas {
    width: 100%;
    height: 100%;
    max-height: none;
  }
  .overlay {
    position: absolute;
    inset: 0 0 auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 8px;
    background: linear-gradient(rgb(0 0 0 / 0.6), transparent);
    color: #fff;
    font-size: 13px;
    opacity: 0;
    transition: opacity 0.15s;
  }
  .tile:hover .overlay,
  .tile:focus-within .overlay {
    opacity: 1;
  }
  .live {
    padding: 0 5px;
    border-radius: 3px;
    background: var(--danger);
    font-size: 10px;
    font-weight: 700;
  }
  .tools button {
    padding: 2px 8px;
    border-radius: 4px;
    color: #fff;
    font-size: 16px;
  }
  .tools button:hover {
    background: rgb(255 255 255 / 0.15);
  }
</style>
