<script lang="ts">
  import type { ComponentProps } from "svelte";
  import ProfilePicture from "./ProfilePicture.svelte";
  import { notificationPermission } from "../lib/notify";
  import type { AccountActions } from "../lib/accounts";
  import { keyLabel, meterPercent, METER_MIN_DB, type Settings } from "../lib/settings";
  import { isDesktop } from "../lib/desktop";
  import { outputDeviceSupported } from "../lib/voice/support";

  let {
    settings,
    level,
    deviceLabel = "",
    fellBack = false,
    onChange,
    onClose,
    account,
    profile,
  }: {
    settings: Settings;
    level: number;
    // The microphone really in use, and whether the chosen one failed.
    deviceLabel?: string;
    fellBack?: boolean;
    onChange: (patch: Partial<Settings>) => void;
    onClose: () => void;
    // Logged in to an account: log out, password.
    account?: AccountActions;
    // Our avatar picture.
    profile?: ComponentProps<typeof ProfilePicture>;
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

  let oldPassword = $state("");
  let newPassword = $state("");
  let passwordNote = $state<string | null>(null);
  const changePassword = async (event: SubmitEvent) => {
    event.preventDefault();
    try {
      await account?.changePassword(oldPassword, newPassword);
      passwordNote = "Password changed. Other devices are logged out.";
      oldPassword = newPassword = "";
    } catch (err) {
      passwordNote = err instanceof Error ? err.message : String(err);
    }
  };

  const recordKey = (event: KeyboardEvent) => {
    if (!recordingKey) return;
    event.preventDefault();
    event.stopPropagation();
    if (event.code !== "Escape") onChange({ pttKey: event.code });
    recordingKey = false;
  };
  const blockedNote =
    typeof Notification === "undefined"
      ? "This browser has no desktop notifications."
      : "The browser blocks notifications for this site.";
  let notificationNote = $state<string | null>(
    typeof Notification === "undefined" || Notification.permission === "denied" ? blockedNote : null,
  );
</script>

<svelte:window onkeydowncapture={recordKey} />

<dialog bind:this={dialog} onclose={onClose} aria-labelledby="settings-title">
  <header>
    <h2 id="settings-title">Settings</h2>
    <button class="close" onclick={() => dialog?.close()} aria-label="Close">×</button>
  </header>

  {#if profile}<ProfilePicture {...profile} />{/if}

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
      <p class="hint">
        {isDesktop()
          ? "Works in the background too. The key is reserved for Kallisti while the app runs."
          : "Works only while this tab has focus."}
      </p>
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
  <label class="check">
    <input
      type="checkbox"
      checked={settings.notifications}
      onchange={async (e) => {
        const box = e.currentTarget;
        let on = box.checked;
        if (on && notificationPermission() !== "granted" && typeof Notification !== "undefined") {
          on = (await Notification.requestPermission()) === "granted";
        }
        notificationNote = on || !box.checked ? null : blockedNote;
        box.checked = on;
        onChange({ notifications: on });
      }}
    />
    Desktop notifications for mentions and private messages
  </label>
  {#if notificationNote}<p class="hint warn" role="status">{notificationNote}</p>{/if}
  {#if account}
    <form class="account" onsubmit={changePassword}>
      <span class="label">Change password</span>
      <input type="password" bind:value={oldPassword} placeholder="Current password" autocomplete="current-password" required />
      <input type="password" bind:value={newPassword} placeholder="New password (8 or more characters)" autocomplete="new-password" minlength="8" required />
      {#if passwordNote}<p class="hint" role="status">{passwordNote}</p>{/if}
      <div class="buttons">
        <button type="submit" class="secondary">Change password</button>
        <span class="spacer"></span>
        <button type="button" class="logout" onclick={account.logoutAll}>Log out everywhere</button>
        <button type="button" class="logout" onclick={account.logout}>Log out</button>
      </div>
    </form>
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
  .check {
    flex-direction: row;
    align-items: center;
    gap: 8px;
    text-transform: none;
    font-weight: normal;
  }
  .account {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding-top: 12px;
    border-top: 1px solid var(--border-subtle);
  }
  .account input {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
  }
  .buttons {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .spacer {
    flex: 1;
  }
  .secondary {
    padding: 8px 14px;
    border-radius: 6px;
    background: var(--bg-input);
    font-weight: 600;
  }
  .logout {
    padding: 8px 14px;
    border-radius: 6px;
    color: var(--danger);
    font-weight: 600;
  }
  .logout:hover {
    background: var(--bg-hover);
  }
  .hint {
    margin: 0;
    text-transform: none;
    font-weight: normal;
    font-size: 12px;
    color: var(--text-faint);
  }
</style>
