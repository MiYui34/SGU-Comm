<script lang="ts">
  import type { ComponentProps } from "svelte";
  import ProfilePicture from "./ProfilePicture.svelte";
  import EmojiList from "./EmojiList.svelte";
  import { BIO_MAX, bioToSave } from "../lib/bio";
  import { notifier, permissionNote, type Permission } from "../lib/notify";
  import { browserPushSupport, pushIsOn, turnOffPush, turnOnPush, type PushPrefs } from "../lib/push";
  import { needsReauth, type AccountActions } from "../lib/accounts";
  import { handleHost } from "../lib/fedi";
  import { keyLabel, meterPercent, METER_MIN_DB, type Settings, type ThemeSetting } from "../lib/settings";
  import { isDesktop } from "../lib/desktop";
  import { outputDeviceSupported } from "../lib/voice/support";
  import { noiseLabel, type NoiseMode } from "../lib/voice/noise";
  import { BUFFERS, bufferLabel, type PlaybackBuffer } from "../lib/voice/buffer";
  import { busyLabel } from "../lib/saveFeedback";
  import { createSave } from "../lib/saveFeedback.svelte";
  import SaveStatus from "./SaveStatus.svelte";
  import FediAccount from "./FediAccount.svelte";

  let {
    settings,
    level,
    deviceLabel = "",
    fellBack = false,
    noiseError = null,
    voiceError = null,
    noiseRunning = null,
    onChange,
    onClose,
    onEcho,
    account,
    push,
    profile,
    bio,
    emoji,
  }: {
    settings: Settings;
    level: number;
    // The microphone really in use, and whether the chosen one failed.
    deviceLabel?: string;
    fellBack?: boolean;
    // Strong noise suppression failed to load (the standard one is on).
    noiseError?: string | null;
    // The browser cannot do voice, and why.
    voiceError?: string | null;
    // Our filter that runs now (null: the browser's, or still loading).
    noiseRunning?: NoiseMode | null;
    onChange: (patch: Partial<Settings>) => void;
    onClose: () => void;
    // The echo test: hear the microphone as others would (after filters).
    onEcho?: (on: boolean) => void;
    // Logged in to an account: log out, password.
    account?: AccountActions;
    // Push notifications for this account (see lib/push).
    push?: { base: string; name: string; prefs: PushPrefs };
    // Our avatar picture.
    profile?: ComponentProps<typeof ProfilePicture>;
    // Our bio as plain text (loading: a long one is not here yet).
    bio?: { text: string; loading: boolean; onSave: (text: string) => void };
    // Our personal emoji (registered users but guests).
    emoji?: ComponentProps<typeof EmojiList>;
  } = $props();

  // Saved when the field loses focus or the settings close. The draft
  // follows changes of the text only (not of the rest of our state), and
  // keeps the last text while a new long one loads (after our own save).
  let lastBio = "";
  const bioText = $derived.by(() => (bio && !bio.loading ? (lastBio = bio.text) : lastBio));
  let bioDraft = $derived(bioText);
  const saveBio = () => {
    const text = bio ? bioToSave(bioDraft, bioText) : null;
    if (text !== null) bio?.onSave(text);
  };

  // The switch shows the browser's subscription; changes wait for it.
  const pushSupport = browserPushSupport();
  let pushOn = $state(false);
  const pushSave = createSave();
  $effect(() => {
    if (push && pushSupport === "ok") void pushIsOn(push.base, push.name).then((on) => (pushOn = on));
  });
  const setPush = (el: HTMLInputElement) => {
    if (!push) return;
    const { base, name, prefs } = push;
    const on = el.checked;
    pushSave.run(
      async () => {
        const note = on ? await turnOnPush(base, name, prefs) : (await turnOffPush(base), null);
        pushOn = await pushIsOn(base, name);
        if (note || pushOn !== on) throw new Error(note ?? "浏览器没有改成。");
      },
      () => (el.checked = pushOn),
    );
  };

  let dialog: HTMLDialogElement | undefined = $state();
  let echo = $state(false);
  $effect(() => {
    onEcho?.(echo);
  });
  $effect(() => () => onEcho?.(false)); // off when the settings close
  let devices = $state<MediaDeviceInfo[]>([]);
  let recordingKey = $state(false);
  const outputSupported = outputDeviceSupported();
  const inputs = $derived(devices.filter((d) => d.kind === "audioinput"));
  const outputs = $derived(devices.filter((d) => d.kind === "audiooutput"));
  // Cameras have no ids before the browser may use one.
  const hasCamera = $derived(devices.some((d) => d.kind === "videoinput"));
  const cameras = $derived(devices.filter((d) => d.kind === "videoinput" && d.deviceId));

  // Device labels are only visible after microphone permission was granted.
  const loadDevices = () =>
    navigator.mediaDevices
      .enumerateDevices()
      .then((list) => (devices = list))
      .catch((err) => console.warn("cannot list devices", err));

  $effect(() => {
    dialog?.showModal();
    void loadDevices();
    navigator.mediaDevices.addEventListener("devicechange", loadDevices);
    return () => navigator.mediaDevices.removeEventListener("devicechange", loadDevices);
  });

  let oldPassword = $state("");
  let newPassword = $state("");
  let passwordNote = $state<string | null>(null); // what changed (errors: passwordSave)
  const passwordSave = createSave();
  // The first password of a fediverse-only account needs a fresh login.
  let reauth = $state(false);
  const reauthSave = createSave();
  const changePassword = (event: SubmitEvent) => {
    event.preventDefault();
    passwordNote = null;
    passwordSave.run(async () => {
      const [kept, first] = [account?.temporary, account?.hasPassword === false];
      await account?.changePassword(oldPassword, newPassword).catch((err: unknown) => {
        reauth = needsReauth(err);
        throw err;
      });
      passwordNote = kept
        ? "Password set: your account stays."
        : first
          ? "Password set: you can also log in with your name. Other devices are logged out."
          : "Password changed. Other devices are logged out.";
      oldPassword = newPassword = "";
    });
  };

  const recordKey = (event: KeyboardEvent) => {
    if (!recordingKey) return;
    event.preventDefault();
    event.stopPropagation();
    if (event.code !== "Escape") onChange({ pttKey: event.code });
    recordingKey = false;
  };
  // Not granted after asking: blocked (or the prompt was dismissed).
  const blockedNote = (p: Permission) => permissionNote(p === "unsupported" ? p : "denied", isDesktop());
  let notificationNote = $state<string | null>(permissionNote(notifier().permission(), isDesktop()));
</script>

<svelte:window onkeydowncapture={recordKey} />

<dialog
  bind:this={dialog}
  onclose={() => {
    saveBio();
    onClose();
  }}
  aria-labelledby="settings-title">
  <header>
    <h2 id="settings-title">设置</h2>
    <button class="close" onclick={() => dialog?.close()} aria-label="关闭">×</button>
  </header>
  {#if voiceError}
    <p class="hint warn voice-error" role="status">{voiceError}</p>
  {/if}

  {#if profile}<ProfilePicture {...profile} />{/if}

  {#if bio}
    <label class="bio">
      <span class="label-row">关于我 <span class="count">{bioDraft.length}/{BIO_MAX}</span></span>
      <textarea rows="3" maxlength={BIO_MAX} disabled={bio.loading && !bioText} bind:value={bioDraft} onblur={saveBio}></textarea>
      <span class="hint">服务器上的人都看得到，Mumble 客户端里也会显示。</span>
    </label>
  {/if}

  {#if emoji}
    <section class="my-emoji" aria-labelledby="my-emoji-title">
      <h3 id="my-emoji-title">我的表情</h3>
      <p class="hint">
        打 :名字: 就能用一个。别人在消息和表态里看得到，Mumble 客户端里显示成 :名字:。想加一个，点消息上的表情即可。
      </p>
      <EmojiList {...emoji} />
    </section>
  {/if}

  <label>
    主题
    <select value={settings.theme} onchange={(e) => onChange({ theme: e.currentTarget.value as ThemeSetting })}>
      <option value="dark">深色</option>
      <option value="light">浅色</option>
      <option value="system">跟随系统</option>
    </select>
  </label>

  <label>
    输入设备
    <select
      value={settings.inputDeviceId ?? ""}
      onchange={(e) => onChange({ inputDeviceId: e.currentTarget.value || null })}
    >
      <option value="">默认</option>
      {#each inputs as d (d.deviceId)}
        {#if d.deviceId !== "default"}
          <option value={d.deviceId}>{d.label || "麦克风"}</option>
        {/if}
      {/each}
    </select>
    {#if fellBack}
      <span class="hint warn" role="status">
        选中的麦克风现在用不了，改用 {deviceLabel || "默认麦克风"}.
      </span>
    {:else if deviceLabel}
      <span class="hint">正在用：{deviceLabel}</span>
    {/if}
  </label>

  {#if hasCamera}
    <label>
      摄像头
      <select value={settings.cameraDeviceId ?? ""} onchange={(e) => onChange({ cameraDeviceId: e.currentTarget.value || null })}>
        <option value="">默认</option>
        {#each cameras as d, i (d.deviceId)}
          <option value={d.deviceId}>{d.label || `摄像头 ${i + 1}`}</option>
        {/each}
      </select>
      {#if !cameras.length}<span class="hint">在语音里打开摄像头之后，这里才会有列表。</span>{/if}
    </label>
  {/if}

  <label>
    降噪
    <select
      value={settings.noiseSuppression}
      disabled={settings.musicMode}
      onchange={(e) => onChange({ noiseSuppression: e.currentTarget.value as NoiseMode })}
    >
      <option value="browser">标准（浏览器自带）</option>
      <option value="strong">强（RNNoise）</option>
      <option value="strongest">最强（DeepFilterNet3）</option>
    </select>
    {#if settings.musicMode}
      <span class="hint">音乐模式下不用。</span>
    {:else if noiseError}
      <span class="hint warn" role="status">{noiseError}</span>
    {:else if noiseRunning && noiseRunning === settings.noiseSuppression}
      <span class="hint" role="status">{noiseLabel(noiseRunning)} 正在运行。</span>
    {:else if settings.noiseSuppression === "strongest"}
      <span class="hint" role="status">正在加载 DeepFilterNet3（约 12 MB，只下一次）……</span>
    {:else}
      <span class="hint">
        「强」会滤掉风扇这类持续噪声，光打字不会触发发送。「最强」连打字声也滤掉，但要下约 12 MB，更费 CPU，会多一点点延迟。
      </span>
    {/if}
  </label>
  {#if onEcho}
    <div class="row">
      <span class="label">听听自己</span>
      <div class="buttons">
        <button type="button" class="secondary" aria-pressed={echo} onclick={() => (echo = !echo)}>
          {echo ? "停止" : "开始回声测试"}
        </button>
      </div>
      <p class="hint">按别人听到的效果放一遍你的麦克风（已经过降噪）。请戴耳机。</p>
    </div>
  {/if}

  {#if outputSupported}
    <label>
      输出设备
      <select
        value={settings.outputDeviceId ?? ""}
        onchange={(e) => onChange({ outputDeviceId: e.currentTarget.value || null })}
      >
        <option value="">默认</option>
        {#each outputs as d (d.deviceId)}
          {#if d.deviceId !== "default"}
            <option value={d.deviceId}>{d.label || "扬声器"}</option>
          {/if}
        {/each}
      </select>
    </label>
  {/if}
  <label>
    播放缓冲
    <select
      value={settings.playbackBuffer}
      onchange={(e) => onChange({ playbackBuffer: e.currentTarget.value as PlaybackBuffer })}
    >
      {#each BUFFERS as b (b)}
        <option value={b}>{bufferLabel(b)}</option>
      {/each}
    </select>
    <span class="hint">
      别人的声音先等多久再放。连接差的时候缓冲长一点能填补空档，但延迟更大。音乐至少 500 ms。
    </span>
  </label>
  <label class="check">
    <input type="checkbox" checked={settings.sounds} onchange={(e) => onChange({ sounds: e.currentTarget.checked })} />
    <span>提示音</span>
    <span class="hint">静音、取消静音、关掉耳机的时候响一下。</span>
  </label>
  {#if settings.sounds}
    <label>
      提示音音量
      <input
        type="range"
        min="0"
        max="1"
        step="0.05"
        value={settings.soundVolume}
        oninput={(e) => onChange({ soundVolume: Number(e.currentTarget.value) })}
        aria-valuetext="{Math.round(settings.soundVolume * 100)}%"
      />
    </label>
  {/if}

  <fieldset>
    <legend>说话方式</legend>
    <label class="radio">
      <input
        type="radio"
        name="mode"
        checked={settings.mode === "vad"}
        onchange={() => onChange({ mode: "vad" })}
      />
      自动（听到说话就开）
    </label>
    <label class="radio">
      <input
        type="radio"
        name="mode"
        checked={settings.mode === "ptt"}
        onchange={() => onChange({ mode: "ptt" })}
      />
      按住某个键
    </label>
  </fieldset>

  {#if settings.mode === "ptt"}
    <div class="row">
      <span class="label">按住说话的键</span>
      <button
        class="key"
        class:recording={recordingKey}
        aria-pressed={recordingKey}
        onclick={() => (recordingKey = true)}
      >
        {recordingKey ? "按下要用的键……（Esc 取消）" : keyLabel(settings.pttKey)}
      </button>
      <p class="hint">
        {isDesktop()
          ? "后台也管用。应用开着的时候这个键归 Kallisti。"
          : "只在这个标签页在前台时管用。"}
      </p>
    </div>
  {:else}
    <div class="row">
      <span class="label">灵敏度</span>
      <div class="meter" aria-hidden="true">
        <div
          class="level"
          class:active={level >= settings.vadThresholdDb}
          style:width="{meterPercent(level)}%"
        ></div>
      </div>
      <input
        type="range"
        min={METER_MIN_DB}
        max="0"
        step="1"
        value={settings.vadThresholdDb}
        oninput={(e) => onChange({ vadThresholdDb: Number(e.currentTarget.value) })}
        aria-label="说话的灵敏度"
        aria-valuetext="{settings.vadThresholdDb} dB"
      />
      <p class="hint">
        正常说话时这条应该变绿，安静的时候是灰的。
      </p>
    </div>
  {/if}
  <label class="check">
    <input
      type="checkbox"
      checked={settings.echoCancellation && !settings.musicMode}
      disabled={settings.musicMode}
      onchange={(e) => onChange({ echoCancellation: e.currentTarget.checked })}
    />
    <span>回声消除</span>
    <span class="hint">用外放时需要。戴耳机可以关掉：Mac、桌面版和 Safari 会把蓝牙耳机切到通话模式，声音更差。</span>
  </label>
  <label class="check">
    <input type="checkbox" checked={settings.musicMode} onchange={(e) => onChange({ musicMode: e.currentTarget.checked })} />
    <span>音乐模式</span>
    <span class="hint">用来放音乐：不降噪、不消回声、不自动调音量，声音更好（麦克风是双声道就出立体声）。请戴耳机。</span>
  </label>
  <label class="check">
    <input
      type="checkbox"
      checked={settings.closeMicWhenMuted}
      onchange={(e) => onChange({ closeMicWhenMuted: e.currentTarget.checked })}
    />
    <span>静音时关掉麦克风</span>
    <span class="hint">浏览器就不再显示正在录音，但静音和取消静音的一瞬间声音会断一下。</span>
  </label>
  <label class="check">
    <input type="checkbox" checked={settings.streamPreviews} onchange={(e) => onChange({ streamPreviews: e.currentTarget.checked })} />
    <span>共享屏幕或摄像头时给别人看小画面</span>
    <span class="hint">没在看的人每隔几秒会收到一张小图。</span>
  </label>
  <label class="check">
    <input
      type="checkbox"
      checked={settings.notifications}
      onchange={async (e) => {
        const box = e.currentTarget;
        const n = notifier();
        let [on, permission] = [box.checked, n.permission()];
        if (on && permission !== "granted" && permission !== "unsupported") permission = await n.request();
        on &&= permission === "granted";
        notificationNote = on || !box.checked ? null : blockedNote(permission);
        box.checked = on;
        onChange({ notifications: on });
      }}
    />
    <span>桌面通知（被提及和私信；每个频道可以在它的菜单里单独设）</span>
    {#if notificationNote}<span class="hint warn" role="status">{notificationNote}</span>{/if}
  </label>
  {#if push && pushSupport === "ok"}
    <label class="check">
      <input type="checkbox" checked={pushOn} disabled={pushSave.busy} onchange={(e) => setPush(e.currentTarget)} />
      <span>在这台设备上收推送</span>
      <span class="hint">应用关着也能收到被提及和私信。只告诉你是谁、在哪，不带内容。</span>
      <SaveStatus save={pushSave.state} />
    </label>
  {:else if push && pushSupport === "install"}
    <p class="hint">
      想在 iPhone 或 iPad 上收推送：点「分享」，选「添加到主屏幕」，再从那里打开 Kallisti（要 iOS 16.4 以上）。
    </p>
  {/if}
  {#if account}
    <form class="account" onsubmit={changePassword}>
      {#if account.temporary}
        <span class="label">保留这个账号</span>
        <p class="hint">
          这是游客账号：最后一次登录之后一天就会被删。设了密码就能留住，还能在别的设备上登录。
        </p>
      {:else if !account.hasPassword}
        <span class="label">设一个密码</span>
        <p class="hint">你现在用 fediverse 账号登录。设了密码之后，也能用这个名字登录 Mumble。先要在 fediverse 服务器上确认一次。</p>
      {:else}
        <span class="label">改密码</span>
        <input type="password" bind:value={oldPassword} placeholder="现在的密码" autocomplete="current-password" required />
      {/if}
      <input type="password" bind:value={newPassword} placeholder="新密码（至少 8 位）" autocomplete="new-password" minlength="8" required />
      {#if passwordNote}<p class="hint" role="status">{passwordNote}</p>{/if}
      <div class="buttons">
        <button type="submit" class="secondary" disabled={passwordSave.busy}>
          {busyLabel(passwordSave.state, account.temporary || !account.hasPassword ? "设密码" : "改密码")}
        </button>
        <SaveStatus save={passwordSave.state} done="" />
      </div>
      <!-- The first password: confirm with the fediverse account first (again
           after a refusal: the confirmation lasts a few minutes, once). Not
           in the desktop app, which cannot come back from the instance. -->
      {#if (reauth || (!account.hasPassword && !account.temporary)) && account.fedi && !isDesktop()}
        <div class="buttons">
          <button type="button" class="secondary" disabled={reauthSave.busy} onclick={() => reauthSave.run(account.fediReauth)}>
            去 {handleHost(account.fedi.handle)} 确认
          </button>
          <SaveStatus save={reauthSave.state} done="" />
        </div>
      {/if}
    </form>
    <FediAccount {account} />
    <div class="account">
      <span class="label">退出登录</span>
      {#if account.temporary}
        <p class="hint">退出会结束这个游客账号（在上面设个密码就能留住）。</p>
      {:else}
        <p class="hint">「所有设备」会把你别的浏览器和设备也退掉，比如用过公用电脑之后。</p>
      {/if}
      <div class="buttons">
        <button type="button" class="logout" onclick={account.logout}>退出登录</button>
        {#if !account.temporary}
          <button type="button" class="logout" onclick={account.logoutAll}>所有设备都退出</button>
        {/if}
      </div>
    </div>
  {/if}
  <footer class="hint">
    Kallisti：一个 Mumble 服务器和客户端。 <a href="https://kallisti.chat" target="_blank" rel="noreferrer">kallisti.chat</a>
  </footer>
</dialog>

<style>
  dialog {
    width: min(440px, 92vw);
    padding: 24px;
    border: none;
    border-radius: 8px;
    background: var(--bg-main);
    color: var(--text);
    accent-color: var(--accent-strong);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.6);
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 16px;
  }
  h2 {
    margin: 0;
    font-size: 18px;
  }
  .close {
    font-size: 24px;
    line-height: 1;
    color: var(--text-muted);
  }
  label,
  .row,
  fieldset {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 0 0 16px;
  }
  label,
  .label,
  legend {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  fieldset {
    border: none;
    padding: 0;
  }
  legend {
    padding: 0;
    margin-bottom: 6px;
  }
  .radio {
    flex-direction: row;
    align-items: center;
    gap: 8px;
    margin: 0;
    font-size: 14px;
    font-weight: normal;
    text-transform: none;
    color: var(--text);
    accent-color: var(--accent-strong);
  }
  select,
  .key {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
    text-transform: none;
  }
  .key {
    text-align: left;
  }
  .key.recording {
    outline: 2px solid var(--accent);
  }
  .meter {
    height: 8px;
    border-radius: 4px;
    background: var(--bg-rail);
    overflow: hidden;
  }
  .level {
    height: 100%;
    background: var(--text-faint);
    transition: width 0.05s linear;
  }
  .level.active {
    background: var(--accent-strong);
  }
  input[type="range"],
  input[type="radio"] {
    accent-color: var(--accent);
  }
  .voice-error {
    margin-bottom: 16px;
    font-size: 14px;
  }
  .hint.warn {
    color: var(--danger);
  }
  /* Checkbox, text, and a hint under the text. */
  .check {
    display: grid;
    grid-template-columns: auto 1fr;
    align-items: center;
    column-gap: 8px;
    row-gap: 2px;
    text-transform: none;
    font-weight: normal;
  }
  .check .hint {
    grid-column: 2;
  }
  .account {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding-top: 12px;
    margin-bottom: 16px;
    border-top: 1px solid var(--border-subtle);
  }
  footer {
    padding-top: 12px;
    border-top: 1px solid var(--border-subtle);
  }
  footer a {
    color: var(--link);
  }
  .account input {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
  }
  .buttons {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .secondary {
    padding: 8px 14px;
    border-radius: 6px;
    background: var(--bg-input);
    font-weight: 600;
  }
  .logout {
    padding: 8px 14px;
    border-radius: 6px;
    background: var(--bg-input);
    color: var(--danger);
    font-weight: 600;
  }
  .logout:hover {
    background: var(--bg-hover);
  }
  .bio {
    padding-bottom: 16px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .label-row {
    display: flex;
    justify-content: space-between;
  }
  .count {
    font-weight: normal;
    color: var(--text-faint);
  }
  textarea {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
    font-size: 14px;
    text-transform: none;
    resize: vertical;
  }
  .hint {
    margin: 0;
    text-transform: none;
    font-weight: normal;
    font-size: 12px;
    color: var(--text-faint);
  }
  .my-emoji {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .my-emoji h3 {
    margin: 0;
    font-size: 14px;
  }
</style>
