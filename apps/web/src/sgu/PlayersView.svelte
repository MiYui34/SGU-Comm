<script lang="ts">
  import { playerColor, type Person, type PersonAction, type Player } from "./types";

  // 玩家页：语音里每个频道有谁，加上游戏里在线的人。
  let {
    people,
    game,
    meName,
    joinedId,
    onJoin,
    actions,
  }: {
    people: Person[];
    game: Player[];
    meName: string;
    joinedId?: string | null;
    // 点「进入」就换到那个频道；没有权限或不需要时不给这个函数。
    onJoin?: (roomId: string) => void;
    // 对这一行的人能做的事；返回空数组就不显示那个按钮。
    actions?: (session: number) => PersonAction[];
  } = $props();

  // 谁的操作菜单摊开着（用会话号记）。
  let menuFor = $state<number | null>(null);

  // people 已经按频道排好，同一个频道的连着，切成段就是分组。
  const groups = $derived.by(() => {
    const out: { id: string; name: string; people: Person[] }[] = [];
    for (const person of people) {
      const last = out[out.length - 1];
      if (last !== undefined && last.id === person.roomId) last.people.push(person);
      else out.push({ id: person.roomId, name: person.roomName, people: [person] });
    }
    return out;
  });

  function initialOf(name: string): string {
    return [...name][0] ?? "?";
  }
</script>

<div class="players">
  <div class="head-row">
    <h2>语音里</h2>
    <span>{people.length} 人</span>
  </div>

  {#if groups.length === 0}
    <p class="empty">现在没有人在语音里</p>
  {:else}
    <div class="list">
      {#each groups as group (group.id)}
        <div class="group">
          <div class="group-head">
            <span class="room">{group.name}</span>
            <span class="count">{group.people.length} 人</span>
            {#if onJoin && group.id !== "0" && group.id !== joinedId}
              <button type="button" class="join" onclick={() => onJoin(group.id)}>进入</button>
            {/if}
          </div>

          {#each group.people as person (person.name)}
            <div class="row">
              <span class="avatar" style:background={playerColor(person.name)}>{initialOf(person.name)}</span>
              <span class="name">{person.name}</span>
              {#if person.name === meName}<span class="you">你</span>{/if}
              {#if person.muted}<span class="state">麦克风已关</span>{/if}
              {#if person.deaf}<span class="state">耳机静音</span>{/if}
              {#if (actions?.(person.session) ?? []).length > 0}
                <div class="ops">
                  <button
                    type="button"
                    class="more"
                    aria-label="对他做什么"
                    onclick={() => (menuFor = menuFor === person.session ? null : person.session)}
                  >
                    ⋯
                  </button>
                  {#if menuFor === person.session}
                    <div class="menu">
                      {#each actions?.(person.session) ?? [] as action (action.key)}
                        <button
                          type="button"
                          onclick={() => {
                            menuFor = null;
                            action.run();
                          }}
                        >
                          {action.label}
                        </button>
                      {/each}
                    </div>
                  {/if}
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {/each}
    </div>
  {/if}

  {#if game.length > 0}
    <div class="head-row">
      <h2>游戏里</h2>
      <span>{game.length} 人</span>
    </div>
    <div class="game">
      {#each game as player (player.id)}
        <span class="chip">
          <span class="avatar small" style:background={playerColor(player.name)}>{initialOf(player.name)}</span>
          {player.name}
        </span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .players {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .ops {
    position: relative;
    margin-left: auto;
  }

  .more {
    padding: 2px 8px;
    font: inherit;
    font-size: 15px;
    line-height: 1;
    color: var(--sg-muted);
    background: none;
    border: 0;
    border-radius: 6px;
    cursor: pointer;
  }

  .more:hover {
    background: var(--sg-panel);
  }

  .menu {
    position: absolute;
    top: 24px;
    right: 0;
    z-index: 5;
    display: flex;
    flex-direction: column;
    min-width: 108px;
    padding: 4px;
    background: var(--sg-card);
    border: 1px solid var(--sg-line);
    border-radius: 9px;
    box-shadow: 0 12px 28px rgb(44 38 29 / 14%);
  }

  .menu button {
    padding: 6px 10px;
    font: inherit;
    font-size: 12.5px;
    color: var(--sg-text);
    text-align: left;
    background: none;
    border: 0;
    border-radius: 6px;
    cursor: pointer;
  }

  .menu button:hover {
    background: var(--sg-panel);
  }

  .head-row {
    display: flex;
    align-items: baseline;
    gap: 10px;
  }

  .head-row + .list,
  .head-row + .game {
    margin-top: -4px;
  }

  h2 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }

  .head-row span {
    color: var(--sg-faint);
    font-size: 12.5px;
  }

  .list {
    overflow: hidden;
    border: 1px solid var(--sg-line);
    border-radius: var(--sg-radius);
    background: var(--sg-card);
  }

  .group + .group {
    border-top: 1px solid var(--sg-line);
  }

  .group-head {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 9px 14px;
    background: var(--sg-panel);
    border-bottom: 1px solid var(--sg-line);
  }

  .room {
    font-size: 12.5px;
    font-weight: 600;
    color: var(--sg-muted);
  }

  .count {
    color: var(--sg-faint);
    font-size: 12px;
  }

  .join {
    margin-left: auto;
    padding: 3px 9px;
    font-size: 12px;
    color: var(--sg-green);
    background: transparent;
    border: 1px solid rgb(31 107 74 / 30%);
    border-radius: 999px;
    cursor: pointer;
  }

  .join:hover {
    background: var(--sg-green-soft);
  }

  .row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 14px;
    font-size: 13px;
  }

  .row + .row {
    border-top: 1px solid rgb(44 38 29 / 6%);
  }

  .avatar {
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    font-size: 11.5px;
    font-weight: 600;
    color: #fdfbf6;
    border-radius: 7px;
    flex: none;
  }

  .avatar.small {
    width: 18px;
    height: 18px;
    font-size: 10px;
    border-radius: 5px;
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .you {
    padding: 0 6px;
    font-size: 11px;
    color: var(--sg-green);
    background: var(--sg-green-soft);
    border-radius: 999px;
  }

  .state {
    margin-left: auto;
    color: var(--sg-faint);
    font-size: 11.5px;
  }

  .state + .state {
    margin-left: 0;
  }

  .game {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px 4px 4px;
    font-size: 12.5px;
    color: var(--sg-muted);
    background: var(--sg-card);
    border: 1px solid var(--sg-line);
    border-radius: 999px;
  }

  .empty {
    margin: 0;
    color: var(--sg-faint);
    font-size: 13px;
  }
</style>
