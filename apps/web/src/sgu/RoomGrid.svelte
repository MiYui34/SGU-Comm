<script lang="ts">
  import type { VoiceRoom } from "./types";

  let {
    rooms,
    joinedId,
    meName,
    onJoin,
    onCreate,
  }: {
    rooms: VoiceRoom[];
    joinedId: string | null;
    meName: string;
    onJoin: (id: string) => void;
    onCreate: () => void;
  } = $props();
</script>

<section class="rooms" aria-label="语音房间">
  {#each rooms as room (room.id)}
    <button type="button" class="room" class:on={room.id === joinedId} onclick={() => onJoin(room.id)}>
      <div class="top">
        <span class="name">{room.name}</span>
        <span class="count">{room.members.length} 人</span>
      </div>

      {#if room.members.length === 0}
        <div class="empty">还没有人</div>
      {:else}
        <div class="members">
          {#each room.members as member, i (member)}
            {#if i > 0}<span class="sep">、</span>{/if}<span class:me-in={member === meName}>{member === meName ? "你" : member}</span>
          {/each}
        </div>
      {/if}

      {#if room.id === joinedId}
        <div class="tag">你在这个房间</div>
      {/if}
    </button>
  {/each}

  <button type="button" class="room create" onclick={onCreate}>
    <span class="plus">＋</span>
    创建房间
  </button>
</section>

<style>
  .rooms {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }

  .room {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-height: 104px;
    padding: 14px 15px;
    border: 1px solid var(--sg-line);
    border-radius: var(--sg-radius);
    background: var(--sg-card);
    text-align: left;
    transition:
      border-color 0.15s,
      background 0.15s;
  }

  .room:hover {
    border-color: var(--sg-line-hi);
    background: var(--sg-card-hi);
  }

  .room.on {
    border-color: rgb(31 107 74 / 45%);
    background: #fbfdfa;
  }

  .top {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .name {
    font-size: 14px;
    font-weight: 600;
  }

  .count {
    margin-left: auto;
    color: var(--sg-faint);
    font-size: 12.5px;
  }

  .room.on .count {
    color: var(--sg-green);
  }

  .members {
    color: var(--sg-muted);
    font-size: 12.5px;
    line-height: 1.7;
  }

  .sep {
    color: var(--sg-faint);
  }

  .me-in {
    color: var(--sg-green);
  }

  .empty {
    color: var(--sg-faint);
    font-size: 12.5px;
  }

  .tag {
    margin-top: auto;
    color: var(--sg-green);
    font-size: 12px;
  }

  .create {
    align-items: center;
    justify-content: center;
    gap: 6px;
    border-style: dashed;
    background: transparent;
    color: var(--sg-faint);
    font-size: 13px;
  }

  .create:hover {
    border-color: var(--sg-green);
    background: transparent;
    color: var(--sg-green);
  }

  .plus {
    font-size: 19px;
    line-height: 1;
  }
</style>
