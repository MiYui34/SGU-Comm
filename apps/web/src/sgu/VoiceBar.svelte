<script lang="ts">
  let {
    roomName,
    count,
    rtt,
    micOn,
    soundOn,
    onToggleMic,
    onToggleSound,
    onLeave,
  }: {
    roomName: string | null;
    count: number;
    rtt: number;
    micOn: boolean;
    soundOn: boolean;
    onToggleMic: () => void;
    onToggleSound: () => void;
    onLeave: () => void;
  } = $props();
</script>

<footer class="bar">
  <button type="button" class="btn" class:off={!micOn} onclick={onToggleMic}>
    {#if micOn}
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">
        <rect x="9" y="2" width="6" height="11" rx="3" />
        <path d="M5 10a7 7 0 0 0 14 0" />
        <path d="M12 19v3" />
      </svg>
      麦克风
    {:else}
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">
        <rect x="9" y="2" width="6" height="11" rx="3" />
        <path d="M5 10a7 7 0 0 0 14 0" />
        <path d="M12 19v3" />
        <path d="M3 3l18 18" />
      </svg>
      已静音
    {/if}
  </button>

  <button type="button" class="btn" class:off={!soundOn} onclick={onToggleSound}>
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">
      <path d="M3 18v-6a9 9 0 0 1 18 0v6" />
      <path d="M21 19a2 2 0 0 1-2 2h-1a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2h3v5Z" />
      <path d="M3 19a2 2 0 0 0 2 2h1a2 2 0 0 0 2-2v-3a2 2 0 0 0-2-2H3v5Z" />
      {#if !soundOn}<path d="M3 3l18 18" />{/if}
    </svg>
    耳机
  </button>

  <div class="sep"></div>

  {#if roomName}
    <div>
      <div class="room">{roomName}</div>
      <div class="sub">{count} 人</div>
    </div>
    <div class="right">
      <span class="ping">{Math.round(rtt)} ms</span>
      <button type="button" class="leave" onclick={onLeave}>离开语音</button>
    </div>
  {:else}
    <div class="idle">还没有加入语音房间</div>
  {/if}
</footer>

<style>
  .bar {
    display: flex;
    grid-area: voice;
    align-items: center;
    gap: 10px;
    padding: 0 22px;
    border-top: 1px solid var(--sg-line);
    background: var(--sg-panel);
  }

  .btn {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    height: 36px;
    padding: 0 13px;
    border: 1px solid var(--sg-line);
    border-radius: 8px;
    background: var(--sg-card);
    color: var(--sg-green);
    font-size: 13px;
  }

  .btn svg {
    width: 16px;
    height: 16px;
  }

  .btn:hover {
    border-color: var(--sg-line-hi);
  }

  .btn.off {
    color: var(--sg-red);
  }

  .sep {
    width: 1px;
    height: 24px;
    margin: 0 6px;
    background: var(--sg-line);
  }

  .room {
    font-size: 13.5px;
    font-weight: 600;
  }

  .sub {
    margin-top: -1px;
    color: var(--sg-faint);
    font-size: 12.5px;
  }

  .idle {
    color: var(--sg-muted);
    font-size: 13px;
  }

  .right {
    display: flex;
    align-items: center;
    gap: 14px;
    margin-left: auto;
  }

  .ping {
    color: var(--sg-muted);
    font-size: 13px;
  }

  .leave {
    height: 36px;
    padding: 0 14px;
    border: 1px solid rgb(184 68 47 / 20%);
    border-radius: 8px;
    background: rgb(184 68 47 / 10%);
    color: var(--sg-red);
    font-size: 13px;
  }

  .leave:hover {
    background: var(--sg-red);
    color: #fff;
  }
</style>
