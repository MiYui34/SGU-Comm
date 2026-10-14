<script lang="ts">
  import { initials } from "../lib/avatar";

  let { serverName }: { serverName: string } = $props();
</script>

<nav class="rail" aria-label="Servers">
  <div class="item">
    <span class="pill active" aria-hidden="true"></span>
    <button class="server active" title={serverName} aria-current="true">
      {initials(serverName)}
    </button>
  </div>
  <hr />
  <div class="item">
    <span class="pill" aria-hidden="true"></span>
    <button class="server add" title="Add a server (not yet)" disabled>+</button>
  </div>
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
  .item {
    position: relative;
    display: flex;
    justify-content: center;
    width: 100%;
  }
  /* The white pill on the left edge marks the current server. */
  .pill {
    position: absolute;
    top: 50%;
    left: 0;
    width: 4px;
    height: 8px;
    border-radius: 0 4px 4px 0;
    background: var(--text);
    opacity: 0;
    transform: translateY(-50%);
    transition:
      height 0.15s,
      opacity 0.15s;
  }
  .item:hover .pill {
    height: 20px;
    opacity: 1;
  }
  .pill.active {
    height: 40px;
    opacity: 1;
  }
  .server {
    width: 40px;
    height: 40px;
    border-radius: 12px;
    background: var(--bg-input);
    font-size: 16px;
    font-weight: 600;
    transition:
      background 0.15s,
      color 0.15s;
  }
  .server.active,
  .server:hover:not(:disabled) {
    background: var(--accent);
    color: white;
  }
  .add {
    color: var(--online);
    font-size: 22px;
    font-weight: 400;
  }
  .item:has(.add:disabled):hover .pill {
    opacity: 0;
  }
  hr {
    width: 24px;
    margin: 0;
    border: none;
    border-top: 2px solid var(--bg-active);
  }
</style>
