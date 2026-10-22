<script lang="ts">
  // 设置页：现在能改的就声音两件事，其余只是把自己的状态说清楚。
  let {
    meName,
    micOn,
    soundOn,
    onToggleMic,
    onToggleSound,
    onOpenSettings,
    onOpenShortcuts,
    onOpenPalette,
    onOpenModeration,
    canModerate = false,
  }: {
    meName: string;
    micOn: boolean;
    soundOn: boolean;
    onToggleMic: () => void;
    onToggleSound: () => void;
    // 上游那套完整的声音设置（设备、降噪、回声消除）。
    onOpenSettings?: () => void;
    // 键位表。
    onOpenShortcuts?: () => void;
    // 搜索与跳转面板（Ctrl K），没有就不显示这一行。
    onOpenPalette?: () => void;
    // 服务器管理（封禁、注册用户、访客只读、验证码），有权限才给。
    onOpenModeration?: () => void;
    canModerate?: boolean;
  } = $props();
</script>

<div class="settings">
  <section>
    <h2>声音</h2>
    <div class="card">
      <div class="row">
        <div class="what">
          <span class="label">麦克风</span>
          <span class="hint">关掉之后别人听不到你</span>
        </div>
        <button
          type="button"
          class="switch"
          class:on={micOn}
          role="switch"
          aria-checked={micOn}
          aria-label="麦克风"
          onclick={onToggleMic}
        >
          <span class="knob"></span>
        </button>
      </div>

      <div class="row">
        <div class="what">
          <span class="label">扬声器</span>
          <span class="hint">关掉之后你听不到别人</span>
        </div>
        <button
          type="button"
          class="switch"
          class:on={soundOn}
          role="switch"
          aria-checked={soundOn}
          aria-label="扬声器"
          onclick={onToggleSound}
        >
          <span class="knob"></span>
        </button>
      </div>
    </div>
  </section>

  <section>
    <h2>账号</h2>
    <div class="card">
      <div class="row">
        <span class="label">名字</span>
        <span class="value">{meName}</span>
      </div>
      <div class="row">
        <span class="label">界面语言</span>
        <span class="value">中文</span>
      </div>
    </div>
  </section>
  <section>
    <h2>更多</h2>
    <div class="card">
      <div class="row">
        <div class="what">
          <span class="label">声音与设备</span>
          <span class="hint">输入输出、降噪、回声消除</span>
        </div>
        <button type="button" class="more" onclick={() => onOpenSettings?.()}>打开</button>
      </div>
      {#if canModerate}
        <div class="row">
          <div class="what">
            <span class="label">服务器管理</span>
            <span class="hint">封禁、注册用户、访客权限、验证码</span>
          </div>
          <button type="button" class="more" onclick={() => onOpenModeration?.()}>打开</button>
        </div>
      {/if}
      <div class="row">
        <div class="what">
          <span class="label">键盘快捷键</span>
          <span class="hint">按住说话、静音、切换频道</span>
        </div>
        <button type="button" class="more" onclick={() => onOpenShortcuts?.()}>查看</button>
      </div>
      {#if onOpenPalette}
        <div class="row">
          <div class="what">
            <span class="label">搜索或跳转</span>
            <span class="hint">Ctrl K 找频道，回车就过去</span>
          </div>
          <button type="button" class="more" onclick={() => onOpenPalette?.()}>打开</button>
        </div>
      {/if}
    </div>
  </section>
</div>

<style>
  .settings {
    display: flex;
    flex-direction: column;
    gap: 18px;
    max-width: 620px;
  }

  h2 {
    margin: 0 0 9px;
    font-size: 15px;
    font-weight: 600;
  }

  .card {
    overflow: hidden;
    background: var(--sg-card);
    border: 1px solid var(--sg-line);
    border-radius: var(--sg-radius);
  }

  .row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 15px;
    font-size: 13px;
  }

  .row + .row {
    border-top: 1px solid var(--sg-line);
  }

  .what {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .label {
    color: var(--sg-text);
  }

  .hint {
    color: var(--sg-faint);
    font-size: 12px;
  }

  .value {
    margin-left: auto;
    color: var(--sg-muted);
  }

  .more {
    margin-left: auto;
    padding: 5px 12px;
    font: inherit;
    font-size: 12.5px;
    color: var(--sg-text);
    background: var(--sg-bg);
    border: 1px solid var(--sg-line-hi);
    border-radius: 8px;
    cursor: pointer;
  }

  .more:hover {
    background: var(--sg-panel);
  }

  .switch {
    position: relative;
    margin-left: auto;
    width: 38px;
    height: 22px;
    padding: 0;
    background: var(--sg-panel);
    border: 1px solid var(--sg-line-hi);
    border-radius: 999px;
    cursor: pointer;
    transition: background 0.15s;
  }

  .switch.on {
    background: var(--sg-green);
    border-color: var(--sg-green);
  }

  .knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 16px;
    height: 16px;
    background: var(--sg-card);
    border-radius: 999px;
    box-shadow: 0 1px 2px rgb(44 38 29 / 20%);
    transition: transform 0.15s;
  }

  .switch.on .knob {
    transform: translateX(16px);
  }
</style>
