<script lang="ts">
  import { qualityOf, type Activity, type ConnectionStats, type ServerInfo } from "./types";

  let {
    connection,
    server,
    activity,
    predicted = null,
  }: {
    connection: ConnectionStats;
    server: ServerInfo;
    activity: Activity[];
    /** 预计游戏延迟（毫秒）：两段都测到才有。 */
    predicted?: number | null;
  } = $props();
</script>

<aside class="rail">
  <div class="card">
    <h3>你的延迟</h3>
    <div class="ping"><b>{Math.round(connection.rtt)}</b><span>ms</span></div>
    <div class="note">{qualityOf(connection.rtt)}</div>
    <div class="extra">
      <div>抖动<b>{connection.jitter.toFixed(1)} ms</b></div>
      <div>丢包<b>{connection.loss.toFixed(1)} %</b></div>
    </div>
    {#if predicted !== null}
      <div class="guess"><span>预计游戏延迟</span><b>{predicted} ms</b></div>
    {/if}
  </div>

  {#if server.slots > 0 || server.uptime || server.version}
    <div class="card">
      <h3>服务器</h3>
      {#if server.slots > 0}
        <div class="kv"><span>在线</span><b>{server.online} / {server.slots}</b></div>
      {/if}
      {#if server.uptime}
        <div class="kv"><span>运行时间</span><b>{server.uptime}</b></div>
      {/if}
      {#if server.version}
        <div class="kv"><span>版本</span><b>{server.version}</b></div>
      {/if}
    </div>
  {/if}

  {#if activity.length > 0}
    <div class="card">
      <h3>最近动态</h3>
      <div class="events">
        {#each activity as item (item.id)}
          <div class="ev">
            <span><span class="k">{item.who}</span> {item.what}</span>
            <time>{item.at}</time>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</aside>

<style>
  .rail {
    display: flex;
    grid-area: rail;
    flex-direction: column;
    gap: 12px;
    min-height: 0;
    overflow-y: auto;
    padding: 20px 16px;
    border-left: 1px solid var(--sg-line);
    background: var(--sg-panel);
  }

  .card {
    padding: 14px 15px;
    border: 1px solid var(--sg-line);
    border-radius: var(--sg-radius);
    background: var(--sg-card);
  }

  h3 {
    margin: 0 0 10px;
    color: var(--sg-muted);
    font-size: 12.5px;
    font-weight: 600;
  }

  .ping {
    display: flex;
    align-items: baseline;
    gap: 5px;
  }

  .ping b {
    font-size: 34px;
    font-weight: 600;
    letter-spacing: -1px;
    line-height: 1;
  }

  .ping span {
    color: var(--sg-muted);
    font-size: 13px;
  }

  .note {
    margin-top: 6px;
    color: var(--sg-faint);
    font-size: 12.5px;
  }

  .extra {
    display: flex;
    gap: 22px;
    margin-top: 14px;
  }

  .extra div {
    color: var(--sg-faint);
    font-size: 12.5px;
  }

  .extra b {
    display: block;
    margin-top: 1px;
    color: var(--sg-text);
    font-size: 14px;
    font-weight: 600;
  }

  .kv {
    display: flex;
    justify-content: space-between;
    padding: 6px 0;
    font-size: 13px;
  }

  .kv span {
    color: var(--sg-muted);
  }

  .guess {
    display: flex;
    justify-content: space-between;
    margin-top: 12px;
    padding-top: 10px;
    border-top: 1px solid var(--sg-line);
    color: var(--sg-muted);
    font-size: 12.5px;
  }

  .guess b {
    color: var(--sg-text);
    font-size: 13px;
    font-weight: 600;
  }

  .kv b {
    font-weight: 500;
  }

  .events {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .ev {
    display: flex;
    color: var(--sg-muted);
    font-size: 12.5px;
    line-height: 1.5;
  }

  .ev .k {
    color: var(--sg-text);
  }

  .ev time {
    flex: none;
    margin-left: auto;
    color: var(--sg-faint);
    font-size: 12px;
  }

</style>
