<script lang="ts">
  import type { Role } from "../lib/roles";
  import type { User } from "@kallisti/protocol";
  import type { UserStatus } from "../lib/presence";
  import Emoji from "./Emoji.svelte";
  import { MAX_VOLUME, type UserAudio } from "../lib/userAudio";
  import { untilText, type ModerationAction, type UserModeration } from "../lib/moderation";
  import Avatar from "./Avatar.svelte";
  import RoleTag from "./RoleTag.svelte";
  import ContextMenu from "./ContextMenu.svelte";
  import { htmlToLines } from "../lib/htmlText";

  let {
    user,
    offline = false,
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
    status = null,
    moderator = null,
    onModerator,
    admin = null,
    onAdmin,
    moderation = null,
    timedOutUntil = null,
    onModerate,
    onConfirm,
    onClose,
  }: {
    user: Pick<User, "name" | "comment" | "mute" | "deaf">;
    // A registered user who is offline: no voice items (volume, mute).
    offline?: boolean;
    isMe: boolean;
    audio: UserAudio;
    x: number;
    y: number;
    onAudio: (patch: Partial<UserAudio>) => void;
    // Their text messages hidden for me (see lib/blocked).
    blocked: boolean;
    onBlock: () => void;
    // None: we cannot write to them (an offline user, we are a guest).
    onMessage?: () => void;
    onSettings: () => void;
    role?: Role | null;
    status?: UserStatus | null;
    // What we may do about the user's role (see moderatorAction,
    // adminAction).
    moderator?: "make" | "remove" | null;
    onModerator?: (on: boolean) => void;
    admin?: "make" | "remove" | null;
    onAdmin?: (on: boolean) => void;
    // What we may do to the user (see userModeration): onModerate acts at
    // once, onConfirm asks first (kick, ban, time-out).
    moderation?: UserModeration | null;
    timedOutUntil?: number | null;
    onModerate?: (a: ModerationAction) => void;
    onConfirm?: (kind: "kick" | "ban" | "timeout") => void;
    onClose: () => void;
  } = $props();

  // The comment as text, never as HTML.
  const bio = $derived(htmlToLines(user.comment));

  const act = (fn: () => void) => () => {
    fn();
    onClose();
  };
</script>

<ContextMenu {x} {y} label="Actions for {user.name}" {onClose}>
  <div class="who">
    <Avatar name={user.name} size={32} status={status?.presence} registered={offline} />
    <span class="names">
      <span class="line"><span>{user.name}</span><RoleTag {role} /></span>
      {#if status?.custom}
        <span class="custom"><Emoji text={status.custom.emoji} /> {status.custom.text}</span>
      {/if}
    </span>
  </div>
  {#if bio}<p class="bio">{bio}</p>{/if}
  <div class="sep"></div>
  {#if isMe}
    <button onclick={act(onSettings)}>Settings</button>
  {:else}
    {#if onMessage}
      <button onclick={act(onMessage)}>Message</button>
      <div class="sep"></div>
    {/if}
    {#if !offline}
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
    {/if}
    <button role="switch" aria-checked={blocked} onclick={onBlock}>
      Block messages
      <span class="box" class:on={blocked} aria-hidden="true"></span>
    </button>
    {#if (moderator && onModerator) || (admin && onAdmin)}
      <div class="sep"></div>
    {/if}
    {#if moderator && onModerator}
      <button onclick={act(() => onModerator(moderator === "make"))}>
        {moderator === "make" ? "Make moderator" : "Remove moderator"}
      </button>
    {/if}
    {#if admin && onAdmin}
      <button onclick={act(() => onAdmin(admin === "make"))}>
        {admin === "make" ? "Make admin" : "Remove admin"}
      </button>
    {/if}
    {#if timedOutUntil !== null}
      <p class="note">Timed out until {untilText(timedOutUntil)}</p>
    {/if}
    {#if moderation && onModerate && onConfirm}
      <div class="sep"></div>
      {#if moderation.mute}
        <!-- A time-out keeps the user muted. -->
        <button role="switch" aria-checked={user.mute} disabled={timedOutUntil !== null} onclick={() => onModerate({ kind: "mute", on: !user.mute })}>
          Server mute
          <span class="box" class:on={user.mute} aria-hidden="true"></span>
        </button>
        <button role="switch" aria-checked={user.deaf} onclick={() => onModerate({ kind: "deaf", on: !user.deaf })}>
          Server deafen
          <span class="box" class:on={user.deaf} aria-hidden="true"></span>
        </button>
      {/if}
      {#if moderation.moveTo.length}
        <label class="volume">
          Move to
          <select value="" onchange={(e) => act(() => onModerate({ kind: "move", channel: Number(e.currentTarget.value) }))()}>
            <option value="" disabled>Choose a voice channel…</option>
            {#each moderation.moveTo as c (c.id)}<option value={String(c.id)}>{c.name}</option>{/each}
          </select>
        </label>
      {/if}
      {#if moderation.timeout}
        {#if timedOutUntil !== null}
          <button onclick={act(() => onModerate({ kind: "timeout", seconds: 0 }))}>Remove time-out</button>
        {:else}
          <button class="danger" onclick={act(() => onConfirm("timeout"))}>Time out…</button>
        {/if}
      {/if}
      {#if moderation.kick}
        <button class="danger" onclick={act(() => onConfirm("kick"))}>Kick…</button>
      {/if}
      {#if moderation.ban}
        <button class="danger" onclick={act(() => onConfirm("ban"))}>Ban…</button>
      {/if}
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
  /* Long ones (from Mumble apps) scroll. */
  .bio {
    max-width: 240px;
    max-height: 120px;
    overflow-y: auto;
    margin: 0;
    padding: 0 8px 6px;
    font-size: 13px;
    white-space: pre-line;
    overflow-wrap: anywhere;
    color: var(--text-muted);
  }
  .names {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .line {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .custom {
    max-width: 220px;
    font-size: 13px;
    font-weight: 400;
    color: var(--text-muted);
    overflow-wrap: anywhere;
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
  select {
    padding: 6px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
    font-size: 14px;
    text-transform: none;
  }
  .note {
    margin: 0;
    padding: 4px 8px;
    font-size: 13px;
    color: var(--text-muted);
  }
  .danger {
    color: var(--danger);
  }
  .danger:hover {
    background: var(--danger);
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
