<script lang="ts">
  import { keyLabel, meterPercent, METER_MIN_DB, type Settings } from "../lib/settings";
  import { outputDeviceSupported } from "../lib/voice/support";

  let {
    settings,
    level,
    deviceLabel = "",
    fellBack = false,
    onChange,
    onClose,
  }: {
    settings: Settings;
    level: number;
    // The microphone really in use, and whether the chosen one failed.
    deviceLabel?: string;
    fellBack?: boolean;
    onChange: (patch: Partial<Settings>) => void;
    onClose: () => void;
  } = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  let devices = $state<MediaDeviceInfo[]>([]);
  let recordingKey = $state(false);
  const outputSupported = outputDeviceSupported();
  const inputs = $derived(devices.filter((d) => d.kind === "audioinput"));
  const outputs = $derived(devices.filter((d) => d.kind === "audiooutput"));

  // Device labels are only visible after microphone permission was granted.
  const loadDevices = () =>
    navigator.mediaDevices
      .enumerateDevices()
      .then((list) => (devices = list))
      .catch((err) => console.warn("cannot list devices", err));

  $effect(() => {
    dialog?.showModal();
    void loadDevices();
    navigator.mediaDevices.addEventListener("devicechange", loadDevices);
    return () => navigator.mediaDevices.removeEventListener("devicechange", loadDevices);
  });

  const recordKey = (event: KeyboardEvent) => {
    if (!recordingKey) return;
    event.preventDefault();
    event.stopPropagation();
    if (event.code !== "Escape") onChange({ pttKey: event.code });
    recordingKey = false;
  };
</script>

<svelte:window onkeydowncapture={recordKey} />

<dialog bind:this={dialog} onclose={onClose} aria-labelledby="settings-title">
  <header>
    <h2 id="settings-title">Voice settings</h2>
    <button class="close" onclick={() => dialog?.close()} aria-label="Close">×</button>
  </header>

  <label>
    Input device
    <select
      value={settings.inputDeviceId ?? ""}
      onchange={(e) => onChange({ inputDeviceId: e.currentTarget.value || null })}
    >
      <option value="">Default</option>
      {#each inputs as d (d.deviceId)}
        {#if d.deviceId !== "default"}
          <option value={d.deviceId}>{d.label || "Microphone"}</option>
        {/if}
      {/each}
    </select>
    {#if fellBack}
      <span class="hint warn" role="status">
        The chosen microphone is not available. Using {deviceLabel || "the default microphone"}.
      </span>
    {:else if deviceLabel}
      <span class="hint">In use: {deviceLabel}</span>
    {/if}
  </label>

  {#if outputSupported}
    <label>
      Output device
      <select
        value={settings.outputDeviceId ?? ""}
        onchange={(e) => onChange({ outputDeviceId: e.currentTarget.value || null })}
      >
        <option value="">Default</option>
        {#each outputs as d (d.deviceId)}
          {#if d.deviceId !== "default"}
            <option value={d.deviceId}>{d.label || "Speakers"}</option>
          {/if}
        {/each}
      </select>
    </label>
  {/if}

  <fieldset>
    <legend>Input mode</legend>
    <label class="radio">
      <input
        type="radio"
        name="mode"
        checked={settings.mode === "vad"}
        onchange={() => onChange({ mode: "vad" })}
      />
      Voice activity
    </label>
    <label class="radio">
      <input
        type="radio"
        name="mode"
        checked={settings.mode === "ptt"}
        onchange={() => onChange({ mode: "ptt" })}
      />
      Push to talk
    </label>
  </fieldset>

  {#if settings.mode === "ptt"}
    <div class="row">
      <span class="label">Push-to-talk key</span>
      <button
        class="key"
        class:recording={recordingKey}
        aria-pressed={recordingKey}
        onclick={() => (recordingKey = true)}
      >
        {recordingKey ? "Press a key… (Esc to cancel)" : keyLabel(settings.pttKey)}
      </button>
      <p class="hint">Works only while this tab has focus.</p>
    </div>
  {:else}
    <div class="row">
      <span class="label">Sensitivity</span>
      <div class="meter" aria-hidden="true">
        <div
          class="level"
          class:active={level >= settings.vadThresholdDb}
          style:width="{meterPercent(level)}%"
        ></div>
      </div>
      <input
        type="range"
        min={METER_MIN_DB}
        max="0"
        step="1"
        value={settings.vadThresholdDb}
        oninput={(e) => onChange({ vadThresholdDb: Number(e.currentTarget.value) })}
        aria-label="Voice activity threshold"
        aria-valuetext="{settings.vadThresholdDb} dB"
      />
      <p class="hint">
        Speak normally: the bar should turn green when you talk, and stay grey when you are quiet.
      </p>
    </div>
  {/if}
</dialog>

<style>
  dialog {
    width: min(440px, 92vw);
    padding: 24px;
    border: none;
    border-radius: 8px;
    background: var(--bg-main);
    color: var(--text);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.6);
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }
  h2 {
    margin: 0;
    font-size: 18px;
  }
  .close {
    font-size: 24px;
    line-height: 1;
    color: var(--text-muted);
  }
  label,
  .row,
  fieldset {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 0 0 16px;
  }
  label,
  .label,
  legend {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  fieldset {
    border: none;
    padding: 0;
  }
  legend {
    padding: 0;
    margin-bottom: 6px;
  }
  .radio {
    flex-direction: row;
    align-items: center;
    gap: 8px;
    margin: 0;
    font-size: 14px;
    font-weight: normal;
    text-transform: none;
    color: var(--text);
  }
  select,
  .key {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
    text-transform: none;
  }
  .key {
    text-align: left;
  }
  .key.recording {
    outline: 2px solid var(--accent);
  }
  .meter {
    height: 8px;
    border-radius: 4px;
    background: var(--bg-rail);
    overflow: hidden;
  }
  .level {
    height: 100%;
    background: var(--text-faint);
    transition: width 0.05s linear;
  }
  .level.active {
    background: var(--accent-strong);
  }
  input[type="range"],
  input[type="radio"] {
    accent-color: var(--accent);
  }
  .hint.warn {
    color: var(--danger);
  }
  .hint {
    margin: 0;
    text-transform: none;
    font-weight: normal;
    font-size: 12px;
    color: var(--text-faint);
  }
</style>
