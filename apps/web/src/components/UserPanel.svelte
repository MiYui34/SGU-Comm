<script lang="ts">
  import type { User } from "@kallisti/protocol";
  import { toggleDeaf, toggleMute, type SelfState } from "../lib/voice";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";

  let {
    me,
    status = "Voice connected",
    active = true,
    micError,
    noVoice = false,
    onSelfState,
    onSettings,
  }: {
    me: User;
    // "Voice connected", "Not in voice", ...
    status?: string;
    // The status in the accent colour (voice is on here).
    active?: boolean;
    micError: string | null;
    // The browser cannot do voice (micError says why).
    noVoice?: boolean;
    onSelfState: (s: SelfState) => void;
    onSettings: () => void;
  } = $props();
</script>

<section class="panel" aria-label="Your status">
  <Avatar name={me.name} size={32} online />
  <div class="who">
    <div class="name ellipsis">{me.name}</div>
    {#if micError}
      <div class="status error ellipsis" title={micError}>{noVoice ? "No voice in this browser" : "No microphone"}</div>
    {:else}
      <div class="status ellipsis" class:idle={!active}>{status}</div>
    {/if}
  </div>
  <button
    title={me.selfMute ? "Unmute" : "Mute"}
    aria-pressed={me.selfMute}
    onclick={() => onSelfState(toggleMute(me))}
  >
    <Icon name="mic" off={me.selfMute} size={20} />
  </button>
  <button
    title={me.selfDeaf ? "Undeafen" : "Deafen"}
    aria-pressed={me.selfDeaf}
    onclick={() => onSelfState(toggleDeaf(me))}
  >
    <Icon name="headphones" off={me.selfDeaf} size={20} />
  </button>
  <button title="Settings" onclick={onSettings}>
    <Icon name="gear" size={20} />
  </button>
</section>

<style>
  .panel {
    --avatar-cutout: var(--bg-input);
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 8px;
    padding: 8px;
    border-radius: 8px;
    background: var(--bg-input);
  }
  .who {
    flex: 1;
    min-width: 0;
  }
  .name {
    font-weight: 600;
    font-size: 14px;
  }
  .status {
    color: var(--accent);
    font-size: 12px;
  }
  .status.error {
    color: var(--danger);
  }
  .status.idle {
    color: var(--text-muted);
  }

  button {
    display: inline-flex;
    padding: 6px;
    border-radius: 6px;
    color: var(--text-muted);
  }
  button:hover {
    background: var(--bg-active);
    color: var(--text);
  }
</style>
