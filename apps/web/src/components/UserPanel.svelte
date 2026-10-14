<script lang="ts">
  import type { User } from "@mumble-discord/protocol";
  import { toggleDeaf, toggleMute, type SelfState } from "../lib/voice";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";

  let { me, onSelfState }: { me: User; onSelfState: (s: SelfState) => void } = $props();
</script>

<section class="panel" aria-label="Your status">
  <Avatar name={me.name} size={32} />
  <div class="who">
    <div class="name">{me.name}</div>
    <div class="status">Connected</div>
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
</section>

<style>
  .panel {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px;
    background: var(--bg-rail);
  }
  .who {
    flex: 1;
    min-width: 0;
  }
  .name {
    font-weight: 600;
    font-size: 14px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .status {
    color: var(--accent);
    font-size: 12px;
  }
  button {
    display: inline-flex;
    padding: 6px;
    border-radius: 4px;
    color: var(--text-muted);
  }
  button:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
</style>
