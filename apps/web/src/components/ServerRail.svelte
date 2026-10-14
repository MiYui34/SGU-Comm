<script lang="ts">
  import { initials } from "../lib/avatar";

  let { serverName }: { serverName: string } = $props();
</script>

<nav class="rail" aria-label="Servers">
  <button class="server active" title={serverName}>{initials(serverName)}</button>
  <hr />
  <button class="server add" title="Add a server (not yet)" disabled>+</button>
</nav>

<style>
  .rail {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 12px 0;
    background: var(--bg-rail);
  }
  .server {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    background: var(--bg-main);
    font-size: 18px;
    font-weight: 600;
    transition: border-radius 0.15s;
  }
  .server:hover:not(:disabled),
  .server.active {
    border-radius: 16px;
    background: var(--accent);
  }
  .add {
    color: var(--accent);
    font-size: 24px;
  }
  .add:disabled {
    cursor: default;
    opacity: 0.6;
  }
  hr {
    width: 32px;
    border: none;
    border-top: 2px solid var(--bg-main);
    margin: 0;
  }
</style>
