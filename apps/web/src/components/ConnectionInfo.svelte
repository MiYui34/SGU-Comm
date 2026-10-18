<script lang="ts">
  import type { ConnectionInfo } from "../lib/connectionInfo.svelte";
  import { connectionLabel, WINDOW_MS } from "../lib/connectionQuality";

  // The signal icon next to "Voice connected", as in Discord; its popup has
  // the numbers. They change with each ping (every 5 s).
  let { info }: { info: ConnectionInfo } = $props();

  const id = $props.id();
  let open = $state(false);
  let root = $state<HTMLElement>();
  let button = $state<HTMLButtonElement>();
  const label = $derived(connectionLabel(info.level, info.rtt));
  // Bars lit: good 3, fair 2, poor 1, not known yet 0.
  const bars = $derived(info.level === "good" ? 3 : info.level === "fair" ? 2 : info.level === "poor" ? 1 : 0);
  const ms = (v: number) => `${Math.round(v)} ms`;

  const close = (e: Event) => {
    if (!open) return;
    if (e instanceof KeyboardEvent && e.key === "Escape") {
      open = false;
      button?.focus();
    } else if (e.type === "pointerdown" && !root?.contains(e.target as Node)) open = false;
  };
</script>

<svelte:window onkeydown={close} onpointerdown={close} />

<span class="conn" bind:this={root}>
  <button
    bind:this={button}
    type="button"
    class="signal {info.level ?? 'unknown'}"
    aria-label={label}
    title={label}
    aria-expanded={open}
    aria-controls={open ? id : undefined}
    onclick={() => (open = !open)}
  >
    <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true">
      {#each [1, 2, 3] as n (n)}
        <rect x={n * 4 - 3} y={13 - n * 4} width="3" height={n * 4} rx="1" class:lit={n <= bars} />
      {/each}
    </svg>
  </button>
  {#if open}
    <div class="popup" {id} role="group" aria-label="Connection">
      <dl>
        <dt>Ping</dt>
        <dd>{info.rtt === null ? "Measuring…" : `${ms(info.rtt)} (± ${ms(info.spread ?? 0)})`}</dd>
        <dt>Audio late or missing</dt>
        <dd>{info.bad === null ? "No audio heard" : `${info.bad.toFixed(1)} %`}</dd>
        <dt>Transport</dt>
        <dd>{info.transport}</dd>
      </dl>
      <p>Over the last {WINDOW_MS / 1000} s.</p>
    </div>
  {/if}
</span>

<style>
  .conn {
    position: relative;
    display: inline-flex;
    vertical-align: middle;
  }
  .signal {
    display: inline-flex;
    padding: 1px;
    border-radius: 4px;
    color: var(--text-faint);
  }
  .signal:hover {
    background: var(--bg-hover);
  }
  .good {
    color: var(--online);
  }
  .fair {
    color: var(--idle);
  }
  .poor {
    color: var(--dnd);
  }
  rect {
    fill: currentColor;
    opacity: 0.3;
  }
  rect.lit {
    opacity: 1;
  }
  .popup {
    position: absolute;
    bottom: calc(100% + 6px);
    left: 0;
    z-index: 20;
    width: max-content;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg-menu);
    color: var(--text);
    font-size: 12px;
    font-weight: 400;
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
  }
  dl {
    display: grid;
    grid-template-columns: auto auto;
    gap: 4px 12px;
    margin: 0;
  }
  dt {
    color: var(--text-muted);
  }
  dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
  }
  p {
    margin: 6px 0 0;
    color: var(--text-faint);
    font-size: 11px;
  }
</style>
