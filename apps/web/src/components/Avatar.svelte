<script lang="ts">
  import { avatarColor, initials } from "../lib/avatar";

  let { name, size = 32 }: { name: string; size?: number } = $props();
</script>

<span
  class="avatar"
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
  }
</style>
