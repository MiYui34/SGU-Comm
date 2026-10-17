<script lang="ts">
  import { avatarColor, initials, pictureFor } from "../lib/avatar";
  import { PRESENCE_LABEL, type Presence } from "../lib/presence";

  // The status dot and speaking ring are cut out of the background behind
  // the avatar: set --avatar-cutout to that color (default: sidebar).
  // status: the dot (none: no dot).
  let {
    name,
    size = 32,
    speaking = false,
    status = null,
    registered = false,
  }: { name: string; size?: number; speaking?: boolean; status?: Presence | null; registered?: boolean } = $props();
  // The user's picture, if the client knows one (see setAvatarPictures).
  const lookup = pictureFor();
  // A picture that did not load (no saved one): initials.
  let failed = $state<string | null>(null);
  const found = $derived(lookup(name, registered));
  const picture = $derived(found === failed ? null : found);
</script>

<span class="wrap" style:width="{size}px" style:height="{size}px">
  <span
    class="avatar"
    class:speaking
    style:font-size="{size * 0.45}px"
    style:background={picture ? undefined : avatarColor(name)}
    >{#if picture}<img src={picture} alt="" onerror={() => (failed = picture)} />{:else}{initials(name)}{/if}</span
  >
  {#if status}
    <span
      class="dot {status}"
      style:--dot="{Math.max(10, Math.round(size * 0.32))}px"
      role="img"
      aria-label={PRESENCE_LABEL[status]}
      title={PRESENCE_LABEL[status]}
    ></span>
  {/if}
</span>

<style>
  .wrap {
    position: relative;
    display: inline-flex;
    flex-shrink: 0;
  }
  .avatar img {
    width: 100%;
    height: 100%;
    border-radius: 50%;
    object-fit: cover;
  }
  .avatar {
    display: inline-flex;
    width: 100%;
    height: 100%;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    color: white;
    font-weight: 600;
    transition: box-shadow 0.1s;
  }
  .speaking {
    box-shadow:
      0 0 0 2px var(--avatar-cutout, var(--bg-sidebar)),
      0 0 0 4px var(--online);
  }
  .dot {
    position: absolute;
    right: -2px;
    bottom: -2px;
    width: var(--dot);
    height: var(--dot);
    border: 3px solid var(--avatar-cutout, var(--bg-sidebar));
    border-radius: 50%;
    background: var(--online);
  }
  /* Idle: a moon; do not disturb: a bar (not only a colour). */
  .dot.idle {
    overflow: hidden;
    background: var(--idle);
  }
  .dot.dnd {
    background: var(--dnd);
  }
  .dot.idle::after,
  .dot.dnd::after {
    content: "";
    position: absolute;
    background: var(--avatar-cutout, var(--bg-sidebar));
  }
  .dot.idle::after {
    top: -15%;
    left: -15%;
    width: 65%;
    height: 65%;
    border-radius: 50%;
  }
  .dot.dnd::after {
    top: 38%;
    left: 20%;
    width: 60%;
    height: 24%;
    border-radius: 2px;
  }
</style>
