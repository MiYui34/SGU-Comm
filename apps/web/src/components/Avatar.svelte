<script lang="ts">
  import { avatarColor, initials } from "../lib/avatar";

  let {
    name,
    size = 32,
    speaking = false,
  }: { name: string; size?: number; speaking?: boolean } = $props();
</script>

<span
  class="avatar"
  class:speaking
  style:width="{size}px"
  style:height="{size}px"
  style:font-size="{size * 0.45}px"
  style:background={avatarColor(name)}>{initials(name)}</span
>

<style>
  .avatar {
    display: inline-flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    color: white;
    font-weight: 600;
    transition: box-shadow 0.1s;
  }
  .speaking {
    box-shadow:
      0 0 0 2px var(--bg-sidebar),
      0 0 0 4px var(--accent);
  }
</style>
