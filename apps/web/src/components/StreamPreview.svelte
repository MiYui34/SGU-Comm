<script lang="ts">
  // The streamer's own screen share, small, with Stop.
  let { stream, onStop }: { stream: MediaStream; onStop: () => void } = $props();
  const show = (video: HTMLVideoElement, s: MediaStream) => {
    video.srcObject = s;
    return { update: (next: MediaStream) => (video.srcObject = next) };
  };
</script>

<section class="preview" aria-label="Your screen share">
  <video use:show={stream} autoplay muted playsinline></video>
  <div class="bar">
    <span class="live">LIVE</span>
    <button onclick={onStop}>Stop</button>
  </div>
</section>

<style>
  .preview {
    margin: 8px 8px 0;
    overflow: hidden;
    border-radius: 8px;
    background: #000;
  }
  video {
    display: block;
    width: 100%;
    max-height: 140px;
    object-fit: contain;
  }
  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 8px;
    background: var(--bg-input);
  }
  .live {
    padding: 1px 6px;
    border-radius: 4px;
    background: var(--danger);
    color: #fff;
    font-size: 11px;
    font-weight: 700;
  }
  button {
    padding: 4px 10px;
    border-radius: 4px;
    background: var(--bg-hover);
    font-size: 13px;
    font-weight: 600;
  }
  button:hover {
    color: var(--danger);
  }
</style>
