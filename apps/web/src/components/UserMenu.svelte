<script lang="ts">
  import type { Role } from "../lib/roles";
  import type { User } from "@kallisti/protocol";
  import { MAX_VOLUME, type UserAudio } from "../lib/userAudio";
  import Avatar from "./Avatar.svelte";
  import RoleTag from "./RoleTag.svelte";
  import ContextMenu from "./ContextMenu.svelte";

  let {
    user,
    isMe,
    audio,
    x,
    y,
    onAudio,
    blocked,
    onBlock,
    onMessage,
    onSettings,
    role = null,
    moderator = null,
    onModerator,
    onClose,
  }: {
    user: User;
    isMe: boolean;
    audio: UserAudio;
    x: number;
    y: number;
    onAudio: (patch: Partial<UserAudio>) => void;
    // Their text messages hidden for me (see lib/blocked).
    blocked: boolean;
    onBlock: () => void;
    onMessage: () => void;
    onSettings: () => void;
    role?: Role | null;
    // What we may do about the user's role (see moderatorAction).
    moderator?: "make" | "remove" | null;
    onModerator?: (on: boolean) => void;
    onClose: () => void;
  } = $props();

  const act = (fn: () => void) => () => {
    fn();
    onClose();
  };
</script>

<ContextMenu {x} {y} label="Actions for {user.name}" {onClose}>
  <div class="who">
    <Avatar name={user.name} size={32} />
    <span>{user.name}</span>
    <RoleTag {role} />
  </div>
  <div class="sep"></div>
  {#if isMe}
    <button onclick={act(onSettings)}>Settings</button>
  {:else}
    <button onclick={act(onMessage)}>Message</button>
    <div class="sep"></div>
    <label class="volume">
      <span class="row">
        User volume <span class="value">{Math.round(audio.volume * 100)}%</span>
      </span>
      <input
        type="range"
        min="0"
        max={MAX_VOLUME * 100}
        step="5"
        value={Math.round(audio.volume * 100)}
        oninput={(e) => onAudio({ volume: Number(e.currentTarget.value) / 100 })}
        aria-valuetext="{Math.round(audio.volume * 100)}%"
      />
    </label>
    <button
      role="switch"
      aria-checked={audio.muted}
      onclick={() => onAudio({ muted: !audio.muted })}
    >
      Mute
      <span class="box" class:on={audio.muted} aria-hidden="true"></span>
    </button>
    <button role="switch" aria-checked={blocked} onclick={onBlock}>
      Block messages
      <span class="box" class:on={blocked} aria-hidden="true"></span>
    </button>
    {#if moderator && onModerator}
      <div class="sep"></div>
      <button onclick={act(() => onModerator(moderator === "make"))}>
        {moderator === "make" ? "Make moderator" : "Remove moderator"}
      </button>
    {/if}
  {/if}
</ContextMenu>

<style>
  .who {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 8px;
    font-weight: 600;
  }
  .sep {
    height: 1px;
    margin: 4px 6px;
    background: var(--border-subtle);
  }
  button {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    padding: 8px;
    border-radius: 4px;
    font-size: 14px;
    text-align: left;
    color: var(--text-muted);
    cursor: pointer;
  }
  button:hover {
    background: var(--accent-strong);
    color: white;
  }
  .volume {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 8px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .row {
    display: flex;
    justify-content: space-between;
  }
  .value {
    color: var(--text);
  }
  input[type="range"] {
    accent-color: var(--accent);
  }
  .box {
    width: 18px;
    height: 18px;
    border: 2px solid var(--text-muted);
    border-radius: 4px;
  }
  .box.on {
    border-color: var(--accent);
    background: var(--accent-strong)
      url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='white' stroke-width='3'%3E%3Cpath d='M5 12l5 5 9-10'/%3E%3C/svg%3E")
      center / 14px no-repeat;
  }
  button:hover .box {
    border-color: white;
  }
</style>
