<script lang="ts">
  import type { Role } from "../lib/roles";
  // A small label for admins and moderators.
  let { role }: { role: Role | null } = $props();
</script>

{#if role}
  <span class="tag" class:admin={role === "admin"}>{role === "admin" ? "Admin" : "Mod"}</span>
{/if}

<style>
  .tag {
    flex: none;
    padding: 1px 5px;
    border-radius: 4px;
    background: var(--accent-strong);
    color: white;
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
  }
  .admin {
    background: var(--danger);
  }
</style>
