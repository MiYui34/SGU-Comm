<script lang="ts">
  import { tick } from "svelte";
  import type { VoiceRoom } from "./types";

  let {
    rooms,
    joinedId,
    meName,
    onJoin,
    onCreate,
    canCreate = true,
  }: {
    rooms: VoiceRoom[];
    joinedId: string | null;
    meName: string;
    onJoin: (id: string) => void;
    // 新建房间：接上真实服务器后真的开一个频道，没接上就只加在页面上。
    onCreate?: (name: string) => void;
    // 服务器上的权限位说不能建就不显示入口，免得点了没反应。
    canCreate?: boolean;
  } = $props();

  let creating = $state(false);
  let draft = $state("");
  let field = $state<HTMLInputElement | null>(null);

  async function startCreating() {
    creating = true;
    draft = "";
    await tick();
    field?.focus();
  }

  function submit(event: SubmitEvent) {
    event.preventDefault();
    const name = draft.trim();
    if (name === "") return;
    onCreate?.(name);
    creating = false;
    draft = "";
  }
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

  {#if canCreate}
    {#if creating}
      <form class="room create on" onsubmit={submit}>
        <input
          bind:this={field}
          bind:value={draft}
          placeholder="房间名字"
          maxlength="24"
          aria-label="房间名字"
        />
        <div class="actions">
          <button type="submit" class="ok" disabled={draft.trim() === ""}>创建</button>
          <button type="button" class="no" onclick={() => (creating = false)}>取消</button>
        </div>
      </form>
    {:else}
      <button type="button" class="room create" onclick={startCreating}>
        <span class="plus">＋</span>
        创建房间
      </button>
    {/if}
  {/if}
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

  .create input {
    width: 100%;
    padding: 7px 9px;
    font: inherit;
    font-size: 13px;
    color: var(--sg-text);
    background: var(--sg-card);
    border: 1px solid var(--sg-line-hi);
    border-radius: 7px;
  }

  .create input:focus {
    outline: none;
    border-color: var(--sg-green);
  }

  .actions {
    display: flex;
    gap: 8px;
  }

  .actions button {
    padding: 5px 12px;
    font: inherit;
    font-size: 12.5px;
    border-radius: 7px;
    cursor: pointer;
  }

  .ok {
    color: #fdfbf6;
    background: var(--sg-green);
    border: 0;
  }

  .ok:disabled {
    color: var(--sg-faint);
    background: var(--sg-panel);
    cursor: default;
  }

  .no {
    color: var(--sg-muted);
    background: transparent;
    border: 1px solid var(--sg-line-hi);
  }
</style>
