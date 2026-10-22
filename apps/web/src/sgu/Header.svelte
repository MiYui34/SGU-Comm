<script lang="ts">
  import type { Me, ServerInfo } from "./types";

  import ConnectionInfo from "../components/ConnectionInfo.svelte";
  import type { ConnectionInfo as Info } from "../lib/connectionInfo.svelte";

  let { server, me, info }: { server: ServerInfo; me: Me; info?: Info | null } = $props();
</script>

<header class="head">
  <div class="mark">SGU</div>
  <div class="brand">SGU 生存服</div>

  <div class="right">
    {#if server.slots > 0}
      <div class="online"><span class="dot"></span><b>{server.online}</b> / {server.slots} 在线</div>
    {:else}
      <div class="off">游戏服未连接</div>
    {/if}
    {#if info}
      <ConnectionInfo info={info} />
    {/if}
    <div class="me">
      <span class="av">{me.initial}</span>
      <span class="name">{me.name}</span>
    </div>
  </div>
</header>

<style>
  .head {
    grid-area: head;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 0 22px;
    border-bottom: 1px solid var(--sg-line);
    background: var(--sg-panel);
  }

  .mark {
    display: grid;
    flex: none;
    place-items: center;
    width: 32px;
    height: 32px;
    border-radius: 8px;
    background: var(--sg-green);
    color: #fff;
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.5px;
  }

  .brand {
    font-size: 15px;
    font-weight: 600;
  }

  .right {
    display: flex;
    align-items: center;
    gap: 18px;
    margin-left: auto;
  }

  .online {
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--sg-muted);
    font-size: 13px;
  }

  .online b {
    color: var(--sg-text);
    font-weight: 600;
  }

  .dot {
    flex: none;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--sg-green);
  }

  .off {
    color: var(--sg-muted);
    font-size: 13px;
    opacity: 0.75;
  }

  .me {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .av {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    border-radius: 7px;
    background: #4a6fa5;
    color: #fff;
    font-size: 11px;
    font-weight: 600;
  }

  .name {
    color: var(--sg-muted);
    font-size: 13px;
  }
</style>
