<script lang="ts">
  import { untrack } from "svelte";
  import Icon from "./Icon.svelte";
  import { gridColumns, nextFocus, stripOf, type StageView } from "../lib/stream/layout";
  import { createPip } from "../lib/stream/pip";

  // The screen shares being watched, above the chat (as Discord's call
  // area): a grid of all, or one in focus over a strip of the others (the
  // strip can be hidden; its streams still decode). Each with its name,
  // picture-in-picture, full screen and close.
  let {
    sessions,
    nameOf,
    attach,
    onClose,
    volumeOf,
    onVolume,
    phone = false,
  }: {
    sessions: number[];
    nameOf: (session: number) => string;
    attach: (session: number, canvas: HTMLCanvasElement | null) => void;
    onClose: (session: number) => void;
    // A stream's sound volume, 0 to 1 (undefined: no sound).
    volumeOf: (session: number) => number | undefined;
    onVolume: (session: number, volume: number) => void;
    phone?: boolean;
  } = $props();

  // Phones start in focus view (a grid of small tiles helps nobody there).
  let view = $state<StageView>(untrack(() => (phone ? "focus" : "grid")));
  let stripHidden = $state(false);
  let focused = $state<number | null>(null);
  let before: number[] = [];
  $effect.pre(() => {
    const now = sessions;
    untrack(() => (focused = nextFocus(focused, before, now)));
    before = now;
  });
  const focusing = $derived(view === "focus" && sessions.length > 1);
  const strip = $derived(focusing ? stripOf(sessions, focused) : []);
  let width = $state(0);
  let height = $state(0);
  const cols = $derived(gridColumns(sessions.length, width, height));
  let stage = $state<HTMLElement>();

  // A click on a stream focuses it; on the focused one: back to the grid.
  const pick = (session: number) => {
    if (sessions.length < 2) return;
    view = focusing && session === focused ? "grid" : "focus";
    focused = session;
  };

  const fullScreen = (el: Element | null | undefined) => {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void el?.requestFullscreen();
  };

  // Picture-in-picture: the canvas's picture in a video element of its own
  // (only videos float). Chromium and Safari; Firefox has no such API, so
  // no button there.
  const pipSupported = document.pictureInPictureEnabled === true && "captureStream" in HTMLCanvasElement.prototype;
  const canvases = new Map<number, HTMLCanvasElement>();
  let pip = $state<number | null>(null);
  const pips = createPip(
    document,
    async (session) => {
      const canvas = canvases.get(session);
      if (!canvas) return null;
      const capture = canvas.captureStream();
      // A still screen draws seldom: a first frame now, so play() does not
      // wait (and the click's permission does not run out).
      (capture.getVideoTracks()[0] as CanvasCaptureMediaStreamTrack | undefined)?.requestFrame();
      const video = document.createElement("video");
      video.muted = true;
      video.srcObject = capture;
      const done = () => {
        capture.getTracks().forEach((t) => t.stop());
        video.srcObject = null;
      };
      video.onleavepictureinpicture = () => {
        done();
        pips.left(video);
      };
      try {
        await video.play();
        await video.requestPictureInPicture();
        return video;
      } catch {
        done();
        return null;
      }
    },
    (session) => (pip = session),
  );

  const canvasOf = (canvas: HTMLCanvasElement, session: number) => {
    canvases.set(session, canvas);
    attach(session, canvas);
    return {
      destroy: () => {
        canvases.delete(session);
        attach(session, null);
        if (pips.current() === session) void document.exitPictureInPicture().catch(() => {});
      },
    };
  };
</script>

<section class="stage" class:focusing class:bare={focusing && stripHidden} bind:this={stage} aria-label="Screen shares">
  {#if sessions.length > 1}
    <div class="bar">
      <button
        onclick={() => (view = view === "grid" ? "focus" : "grid")}
        title={view === "grid" ? "Focus view" : "Grid view"}
        aria-label={view === "grid" ? "Focus view" : "Grid view"}
      >
        <Icon name={view === "grid" ? "focus" : "grid"} />
      </button>
      {#if focusing}
        <button onclick={() => (stripHidden = !stripHidden)} aria-pressed={stripHidden} title={stripHidden ? "Show the others" : "Hide the others"}>
          {stripHidden ? "Show others" : "Hide others"}
        </button>
      {/if}
      <button onclick={() => fullScreen(stage)} title="Full screen" aria-label="Full screen, all streams">
        <Icon name="fullscreen" />
      </button>
    </div>
  {/if}
  <div class="tiles" class:many={sessions.length > 1} bind:clientWidth={width} bind:clientHeight={height} style:--cols={cols} style:--strip={strip.length}>
    {#each sessions as session (session)}
      {@const at = strip.indexOf(session)}
      <div class="tile" class:focused={focusing && session === focused} style:grid-column={at >= 0 ? at + 2 : null}>
        <button class="screen" onclick={() => pick(session)} tabindex={sessions.length > 1 ? 0 : -1} aria-label={focusing && session === focused ? "Grid view" : `Focus ${nameOf(session)}'s screen`}>
          <canvas use:canvasOf={session} aria-label="{nameOf(session)}'s screen"></canvas>
        </button>
        <div class="overlay">
          <span class="who"><span class="live">LIVE</span> {nameOf(session)}</span>
          <span class="tools">
            {#if volumeOf(session) !== undefined}
              <input
                type="range"
                min="0"
                max="1"
                step="0.05"
                value={volumeOf(session)}
                aria-label="Volume of {nameOf(session)}'s stream"
                oninput={(e) => onVolume(session, Number(e.currentTarget.value))}
              />
            {/if}
            {#if pipSupported}
              <button onclick={() => pips.toggle(session)} aria-pressed={pip === session} title="Picture in picture" aria-label="Picture in picture">
                <Icon name="pip" size={16} />
              </button>
            {/if}
            <button onclick={(e) => fullScreen(e.currentTarget.closest(".tile"))} title="Full screen" aria-label="Full screen">⛶</button>
            <button onclick={() => onClose(session)} title="Stop watching" aria-label="Stop watching {nameOf(session)}">×</button>
          </span>
        </div>
      </div>
    {/each}
  </div>
</section>

<style>
  .stage {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 8px 16px;
    background: #000;
  }
  .bar {
    display: flex;
    justify-content: flex-end;
    gap: 4px;
  }
  .tiles {
    display: grid;
    grid-template-columns: repeat(var(--cols), minmax(0, 1fr));
    grid-auto-rows: minmax(0, 1fr);
    gap: 8px;
    height: 55vh;
  }
  /* Focus: the focused stream over a centred strip of the others (their
     columns are set inline, between the two flexible ones). */
  .focusing .tiles {
    grid-template-columns: 1fr repeat(var(--strip), minmax(0, 160px)) 1fr;
    grid-template-rows: minmax(0, 1fr) 90px;
  }
  .focusing .tile.focused {
    grid-row: 1;
    grid-column: 1 / -1;
  }
  .focusing .tile:not(.focused) {
    grid-row: 2;
  }
  .bare .tiles {
    grid-template-rows: minmax(0, 1fr);
  }
  .bare .tile:not(.focused),
  .focusing .tile:not(.focused) .tools input {
    display: none;
  }
  .stage:fullscreen .tiles {
    flex: 1;
    height: auto;
  }
  .tile {
    position: relative;
    min-width: 0;
    min-height: 0;
    background: #000;
  }
  .screen {
    display: block;
    width: 100%;
    height: 100%;
    padding: 0;
    cursor: default;
  }
  .many .screen {
    cursor: pointer;
  }
  canvas {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: contain;
  }
  .overlay {
    position: absolute;
    inset: 0 0 auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 4px;
    padding: 6px 8px;
    background: linear-gradient(rgb(0 0 0 / 0.6), transparent);
    color: #fff;
    font-size: 13px;
    opacity: 0;
    transition: opacity 0.15s;
    pointer-events: none; /* clicks go to the screen below */
  }
  .who {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
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
  .tools {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .tools input {
    width: 90px;
    accent-color: var(--accent);
  }
  .tools button,
  .bar button {
    display: flex;
    align-items: center;
    padding: 2px 8px;
    border-radius: 4px;
    color: #fff;
    font-size: 16px;
  }
  .bar button {
    font-size: 13px;
  }
  .tools button:hover,
  .bar button:hover {
    background: rgb(255 255 255 / 0.15);
  }
  @media (max-width: 700px) {
    .stage {
      padding: 8px;
    }
    .tiles {
      height: 40vh;
    }
    .focusing .tiles {
      grid-template-rows: minmax(0, 1fr) 64px;
    }
  }
</style>
