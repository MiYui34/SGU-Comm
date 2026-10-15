<script lang="ts">
  import { avatarColor, initials, pictureFor } from "../lib/avatar";

  // The status dot and speaking ring are cut out of the background behind
  // the avatar: set --avatar-cutout to that color (default: sidebar).
  let {
    name,
    size = 32,
    speaking = false,
    online = false,
  }: { name: string; size?: number; speaking?: boolean; online?: boolean } = $props();
  // The user's picture, if the client knows one (see setAvatarPictures).
  const lookup = pictureFor();
  const picture = $derived(lookup(name));
</script>

<span class="wrap" style:width="{size}px" style:height="{size}px">
  <span
    class="avatar"
    class:speaking
    style:font-size="{size * 0.45}px"
    style:background={picture ? undefined : avatarColor(name)}
    >{#if picture}<img src={picture} alt="" />{:else}{initials(name)}{/if}</span
  >
  {#if online}
    <span class="dot" style:--dot="{Math.max(10, Math.round(size * 0.32))}px"></span>
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
</style>
