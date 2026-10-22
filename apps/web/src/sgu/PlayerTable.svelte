<script lang="ts">
  import { formatMinutes, type Player } from "./types";

  let { players, detail }: { players: Player[]; detail: boolean } = $props();
</script>

<div class="table" class:compact={!detail}>
  <div class="tr th">
    <span>玩家</span>
    {#if detail}
      <span>位置</span>
      <span class="right">在线时长</span>
    {/if}
  </div>

  {#each players as player (player.id)}
    <div class="tr">
      <div class="who">
        <span class="av" style:background={player.color}>{player.name.slice(0, 1)}</span>
        <span class="n">{player.name}</span>
      </div>
      {#if detail}
        <span class="place">{player.dimension}</span>
        <span class="time">{player.minutes === undefined ? "" : formatMinutes(player.minutes)}</span>
      {/if}
    </div>
  {/each}

  {#if players.length === 0}
    <p class="empty">现在没有人</p>
  {/if}
</div>

<style>
  .table {
    overflow: hidden;
    border: 1px solid var(--sg-line);
    border-radius: var(--sg-radius);
    background: var(--sg-card);
  }

  .tr {
    display: grid;
    grid-template-columns: minmax(0, 1.6fr) minmax(0, 1.4fr) 120px;
    align-items: center;
    gap: 12px;
    padding: 11px 16px;
    border-bottom: 1px solid var(--sg-line);
    font-size: 13px;
  }

  /* 只有名字时不摆空列。 */
  .compact .tr {
    grid-template-columns: minmax(0, 1fr);
  }

  .empty {
    margin: 0;
    padding: 14px 16px;
    color: var(--sg-faint);
    font-size: 13px;
  }

  .tr:last-child {
    border-bottom: none;
  }

  .tr.th {
    padding: 9px 16px;
    background: rgb(44 38 29 / 2.5%);
    color: var(--sg-faint);
    font-size: 12px;
  }

  .tr:not(.th):hover {
    background: rgb(44 38 29 / 2.5%);
  }

  .who {
    display: flex;
    align-items: center;
    gap: 9px;
    min-width: 0;
  }

  .av {
    display: grid;
    flex: none;
    place-items: center;
    width: 26px;
    height: 26px;
    border-radius: 7px;
    color: #fff;
    font-size: 11px;
    font-weight: 600;
  }

  .n {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .place {
    color: var(--sg-muted);
    font-size: 12.5px;
  }

  .time {
    color: var(--sg-muted);
    font-size: 12.5px;
    font-variant-numeric: tabular-nums;
    text-align: right;
  }

  .right {
    text-align: right;
  }
</style>
