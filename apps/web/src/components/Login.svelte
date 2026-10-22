<script lang="ts">
  import { untrack } from "svelte";
  import { modeForError, type Account, type AccountsApi } from "../lib/accounts";
  import { solveInWorker } from "../lib/captcha";
  import { formatFingerprint, type CertificateProblem } from "../lib/desktop";
  import type { Credentials } from "../lib/connection.svelte";
  import { safeGet, safeSet } from "../lib/storage";

  let {
    error,
    api,
    account,
    onAccountChange,
    onConnect,
    fediSignup = false,
    onFediSignupDone,
    desktop = null,
    certificate = null,
    onTrust,
    onCancelTrust,
  }: {
    // Desktop: the server's certificate needs the user's decision.
    certificate?: (CertificateProblem & { server: string }) | null;
    onTrust?: () => void;
    onCancelTrust?: () => void;
    // The desktop app: first the server address (onServer finds out what it
    // is); a Kallisti server then works as on the web, a Mumble server with
    // a name and a password (see lib/desktop).
    desktop?: { server: string; kind: "kallisti" | "mumble" | null; onServer: (address: string) => Promise<void>; onChangeServer: () => void } | null;
    error: string | null;
    api: AccountsApi;
    // The logged-in account (null: none, or a server without accounts).
    account: Account | null;
    onAccountChange: (account: Account | null) => void;
    onConnect: (c: Credentials) => void;
    // Back from a fediverse login without an account here: choose a name
    // (see lib/fedi). onFediSignupDone: made, or given up.
    fediSignup?: boolean;
    onFediSignupDone?: () => void;
  } = $props();

  type Mode = "guest" | "login" | "create";
  const MODES: [Mode, string][] = [
    ["guest", "游客"],
    ["login", "登录"],
    ["create", "注册"],
  ];
  // A guest who tried an account's name gets the Log in tab right away.
  let mode = $state<Mode>(untrack(() => modeForError(error)));
  let username = $state(safeGet(localStorage, "username") ?? "");
  let password = $state("");
  let serverPassword = $state("");
  let server = $state(untrack(() => desktop?.server) ?? "");
  // Desktop and a plain Mumble server: no accounts, one password field.
  const mumbleOnly = $derived(desktop?.kind === "mumble");
  const pickServer = async (event: SubmitEvent) => {
    event.preventDefault();
    if (!desktop || !server.trim()) return;
    busy = true;
    await desktop.onServer(server.trim());
    busy = false;
  };
  let busy = $state(false);
  let checking = $state(false);
  let apiError = $state<string | null>(null);

  // The server's browser check, where it has one: by itself, a second or
  // two (a failure is an error; submitting again retries).
  const prove = async () => {
    const challenge = await api.challenge();
    if (!challenge) return undefined;
    checking = true;
    try {
      return await solveInWorker(challenge);
    } finally {
      checking = false;
    }
  };

  const connectAs = (name: string, isAccount: boolean, pw = serverPassword) => {
    safeSet(localStorage, "username", name);
    onConnect({ username: name, password: pw, account: isAccount, ...(desktop && { server: desktop.server }) });
  };

  const submit = async (event: SubmitEvent) => {
    event.preventDefault();
    const name = username.trim();
    if (!name) return;
    if (mumbleOnly) return connectAs(name, false, password);
    busy = true;
    apiError = null;
    try {
      // A guest gets a temporary account where the server has them (its
      // name is kept, and it can keep the account with a password).
      if (mode === "guest") {
        // A browser that cannot do the check joins as a classic guest (as
        // Mumble clients do).
        const proof = await prove().catch(() => null);
        const guest = proof === null ? null : await api.guest(name, proof);
        if (!guest) return connectAs(name, false);
        onAccountChange(guest);
        return connectAs(guest.name, true);
      }
      const result =
        mode === "login" ? await api.login(name, password) : { name: await api.create(name, password, await prove()), mumble: false };
      const pw = password;
      password = "";
      // A Mumble registration (SuperUser, for example) logs in to Mumble with
      // its password.
      if (result.mumble) return connectAs(result.name, false, pw);
      // With its password state and fediverse link, for the settings.
      onAccountChange((await api.me()) ?? { name: result.name, temporary: false });
      connectAs(result.name, true);
    } catch (err) {
      apiError = err instanceof Error ? err.message : String(err);
    } finally {
      busy = false;
    }
  };

  // Fediverse logins: the server's switch; the instance from last time.
  let fediOn = $state(false);
  $effect(() => {
    if (!desktop) void api.fediEnabled().then((on) => (fediOn = on));
  });
  let instance = $state(safeGet(localStorage, "fediInstance") ?? "");
  const fediLogin = async (event: SubmitEvent) => {
    event.preventDefault();
    if (!instance.trim()) return;
    busy = true;
    apiError = null;
    try {
      const url = await api.fediStart(instance.trim());
      safeSet(localStorage, "fediInstance", instance.trim());
      location.assign(url); // to the instance, and back to the server
    } catch (err) {
      apiError = err instanceof Error ? err.message : String(err);
      busy = false;
    }
  };
  // The name step: the server suggests the fediverse user name.
  let signup = $state<{ name: string; handle: string } | null | undefined>(undefined);
  $effect(() => {
    if (fediSignup && signup === undefined)
      void api.fediSignup().then((s) => {
        signup = s;
        if (s) username = s.name;
      });
  });
  const createFromFedi = async (event: SubmitEvent) => {
    event.preventDefault();
    const name = username.trim();
    if (!name || !signup) return;
    busy = true;
    apiError = null;
    try {
      const made = await api.fediCreate(name, await prove());
      onAccountChange({ name: made, temporary: false, password: false, fedi: { handle: signup.handle, public: false } });
      onFediSignupDone?.();
      connectAs(made, true);
    } catch (err) {
      apiError = err instanceof Error ? err.message : String(err);
    } finally {
      busy = false;
    }
  };

  const logout = async () => {
    if (await api.logout()) onAccountChange(null);
    else apiError = "Logging out did not finish: you may still be logged in. Try again.";
  };
</script>

<div class="screen">
  {#if certificate}
    <div class="card">
      {#if certificate.kind === "unknown"}
        <h1>新服务器</h1>
        <p class="sub">
          第一次连 <strong>{certificate.server}</strong>，它的证书指纹是：
        </p>
      {:else}
        <h1>证书变了</h1>
        <p class="error" role="alert">
          服务器的证书和你上次信任的不一样。可能有人在中间监听。只有管理员确认换了证书，才继续。
        </p>
      {/if}
      <code class="fingerprint">{formatFingerprint(certificate.fingerprint)}</code>
      <p class="hint">方便的话，跟服务器管理员核对一下。</p>
      {#if certificate.kind === "unknown"}
        <button class="primary" onclick={onTrust}>信任并连接</button>
        <button class="link" onclick={onCancelTrust}>取消</button>
      {:else}
        <!-- A changed certificate: the safe choice is the main button. -->
        <button class="primary" onclick={onCancelTrust}>取消</button>
        <button class="link danger" onclick={onTrust}>仍然信任新证书</button>
      {/if}
    </div>
  {:else if account}
    <div class="card">
      <h1>{(apiError ?? error) ? "连不上服务器" : "欢迎回来"}</h1>
      {@render serverLine()}
      <p class="sub">
        {account.temporary ? "当前是游客身份" : "已登录"} <strong>{account.name}</strong>
      </p>
      {#if apiError ?? error}<p class="error" role="alert">{apiError ?? error}</p>{/if}
      <!-- Shown after a failed connection (a saved session connects by
           itself, see App): again by hand, not in a loop. -->
      <button class="primary" onclick={() => connectAs(account.name, true)}>再试一次</button>
      <button class="link" onclick={logout}>退出登录</button>
    </div>
  {:else if fediSignup}
    {#if signup}
      <form class="card" onsubmit={createFromFedi}>
        <h1>起个名字</h1>
        <p class="sub">欢迎，<strong>{signup.handle}</strong>。这个名字别人都会看到。</p>
        <label>
          名字
          <input bind:value={username} required maxlength="32" autocomplete="username" />
        </label>
        {#if apiError}<p class="error" role="alert">{apiError}</p>{/if}
        {#if checking}<p class="hint" role="status">正在检查浏览器…</p>{/if}
        <button class="primary" type="submit" disabled={busy}>注册</button>
        <button type="button" class="link" onclick={onFediSignupDone}>取消</button>
      </form>
    {:else}
      <div class="card">
        <h1>用 fediverse 账号登录</h1>
        <p class="sub" role="status">{signup === null ? "登录超时了，再试一次。" : "正在加载…"}</p>
        {#if signup === null}<button class="primary" onclick={onFediSignupDone}>返回</button>{/if}
      </div>
    {/if}
  {:else}
    {#if desktop && !desktop.kind}
    <form class="card" onsubmit={pickServer}>
      <h1>欢迎来到 SGU 语音</h1>
      <label>
        服务器地址
        <input bind:value={server} required placeholder="chat.example.org" autocomplete="url" />
      </label>
      <p class="hint">填 Kallisti 服务器，或任意 Mumble 服务器（voice.example.org:64738）。</p>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button class="primary" type="submit" disabled={busy}>{busy ? "正在连接…" : "继续"}</button>
    </form>
    {:else}
    <form class="card" onsubmit={submit}>
      <h1>欢迎来到 SGU 语音</h1>
      {@render serverLine()}
      {#if !mumbleOnly}
      <div class="tabs" role="group" aria-label="加入方式">
        {#each MODES as [value, label] (value)}
          <button
            type="button"
            aria-pressed={mode === value}
            class:active={mode === value}
            onclick={() => {
              mode = value;
              apiError = null;
            }}>{label}</button
          >
        {/each}
      </div>
      {/if}
      <label>
        名字
        <input bind:value={username} required maxlength="32" autocomplete="username" />
      </label>
      {#if mumbleOnly}
        <label>
          密码 <span class="optional">（服务器或名字有密码时才填）</span>
          <input bind:value={password} type="password" autocomplete="current-password" />
        </label>
      {:else if mode !== "guest"}
        <label>
          密码
          <input
            bind:value={password}
            type="password"
            required
            minlength={mode === "create" ? 8 : undefined}
            autocomplete={mode === "create" ? "new-password" : "current-password"}
          />
        </label>
      {/if}
      {#if !mumbleOnly}
      <label>
        服务器密码 <span class="optional">（服务器设了才填）</span>
        <input bind:value={serverPassword} type="password" autocomplete="off" />
      </label>
      {/if}
      {#if apiError ?? error}<p class="error" role="alert">{apiError ?? error}</p>{/if}
      {#if mode === "create"}
        <p class="hint">注册之后这个名字就归你了，别人不能用。</p>
      {/if}
      {#if checking}<p class="hint" role="status">正在检查浏览器…</p>{/if}
      <button class="primary" type="submit" disabled={busy}>
        {mumbleOnly ? "连接" : mode === "guest" ? "游客加入" : mode === "login" ? "登录" : "注册"}
      </button>
    </form>
    {/if}
    {#if fediOn && !desktop}
      <form class="card fedi" onsubmit={fediLogin}>
        <label>
          也可以用 fediverse 账号
          <input bind:value={instance} placeholder="mastodon.social or @you@mastodon.social" autocomplete="url" aria-describedby="fedi-hint" />
        </label>
        <p class="hint" id="fedi-hint">支持 Mastodon、Pleroma、Akkoma、GoToSocial 等。第一次来还要再起个名字。</p>
        <button class="secondary" type="submit" disabled={busy || !instance.trim()}>用 fediverse 登录</button>
      </form>
    {/if}
  {/if}
</div>

<!-- Desktop: the chosen server, with a way back to the address step. -->
{#snippet serverLine()}
  {#if desktop}
    <p class="server">
      <span class="ellipsis">{desktop.server}{mumbleOnly ? "（Mumble 服务器）" : ""}</span>
      <button type="button" class="link" onclick={desktop.onChangeServer}>换一个</button>
    </p>
  {/if}
{/snippet}

<style>
  .server {
    display: flex;
    gap: 8px;
    align-items: center;
    justify-content: space-between;
    margin: 0 0 12px;
    color: var(--text-muted);
    font-size: 14px;
  }
  .screen {
    display: flex;
    flex-direction: column;
    gap: 16px;
    align-items: center;
    justify-content: safe center;
    overflow-y: auto;
    height: 100%;
    padding: var(--safe-top) var(--safe-right) var(--safe-bottom) var(--safe-left);
    background: var(--bg-rail);
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 16px;
    width: min(400px, 90vw);
    padding: 32px;
    border-radius: 8px;
    background: var(--bg-main);
  }
  h1 {
    margin: 0;
    font-size: 24px;
    text-align: center;
  }
  .sub {
    margin: -8px 0 8px;
    color: var(--text-muted);
    text-align: center;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .optional {
    text-transform: none;
    font-weight: normal;
  }
  input {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    outline: none;
    /* Not the label's small caps; 16px also keeps phones from zooming in. */
    font-size: 16px;
    font-weight: normal;
    color: var(--text);
  }
  .primary {
    padding: 12px;
    border-radius: 4px;
    background: var(--accent-strong);
    color: white;
    font-weight: 600;
  }
  .secondary {
    padding: 10px;
    border-radius: 4px;
    background: var(--bg-rail);
    font-weight: 600;
  }
  .card.fedi {
    gap: 10px;
    padding: 20px 32px;
  }
  .link {
    color: var(--text-muted);
    font-size: 14px;
  }
  .link.danger {
    color: var(--danger);
  }
  .link:hover {
    color: var(--text);
    text-decoration: underline;
  }
  .tabs {
    display: flex;
    gap: 4px;
    padding: 4px;
    border-radius: 8px;
    background: var(--bg-rail);
  }
  .tabs button {
    flex: 1;
    padding: 8px;
    border-radius: 6px;
    color: var(--text-muted);
    font-size: 14px;
    font-weight: 600;
  }
  .tabs button.active {
    background: var(--bg-active);
    color: var(--text);
  }
  .fingerprint {
    padding: 10px;
    border-radius: 6px;
    background: var(--bg-rail);
    font-size: 13px;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }
  .hint {
    margin: 0;
    color: var(--text-muted);
    font-size: 13px;
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
</style>
