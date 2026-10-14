<script lang="ts">
  import { untrack } from "svelte";
  import { modeForError, type AccountsApi } from "../lib/accounts";
  import type { Credentials } from "../lib/connection.svelte";
  import { safeGet, safeSet } from "../lib/storage";

  let {
    error,
    api,
    account,
    onAccountChange,
    onConnect,
  }: {
    error: string | null;
    api: AccountsApi;
    // The logged-in account (null: none, or a server without accounts).
    account: string | null;
    onAccountChange: (name: string | null) => void;
    onConnect: (c: Credentials) => void;
  } = $props();

  type Mode = "guest" | "login" | "create";
  const MODES: [Mode, string][] = [
    ["guest", "Guest"],
    ["login", "Log in"],
    ["create", "Create account"],
  ];
  // A guest who tried an account's name gets the Log in tab right away.
  let mode = $state<Mode>(untrack(() => modeForError(error)));
  let username = $state(safeGet(localStorage, "username") ?? "");
  let password = $state("");
  let serverPassword = $state("");
  let busy = $state(false);
  let apiError = $state<string | null>(null);

  const connectAs = (name: string, isAccount: boolean) => {
    safeSet(localStorage, "username", name);
    onConnect({ username: name, password: serverPassword, account: isAccount });
  };

  const submit = async (event: SubmitEvent) => {
    event.preventDefault();
    const name = username.trim();
    if (!name) return;
    if (mode === "guest") return connectAs(name, false);
    busy = true;
    apiError = null;
    try {
      const accountName = mode === "login" ? await api.login(name, password) : await api.create(name, password);
      password = "";
      onAccountChange(accountName);
      connectAs(accountName, true);
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
  {#if account}
    <div class="card">
      <h1>Welcome back</h1>
      <p class="sub">Logged in as <strong>{account}</strong></p>
      {#if apiError ?? error}<p class="error" role="alert">{apiError ?? error}</p>{/if}
      <button class="primary" onclick={() => connectAs(account, true)}>Continue as {account}</button>
      <button class="link" onclick={logout}>Log out</button>
    </div>
  {:else}
    <form class="card" onsubmit={submit}>
      <h1>Welcome to Kallisti</h1>
      <div class="tabs" role="group" aria-label="How to join">
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
      <label>
        Name
        <input bind:value={username} required maxlength="32" autocomplete="username" />
      </label>
      {#if mode !== "guest"}
        <label>
          Password
          <input
            bind:value={password}
            type="password"
            required
            minlength={mode === "create" ? 8 : undefined}
            autocomplete={mode === "create" ? "new-password" : "current-password"}
          />
        </label>
      {/if}
      <label>
        Server password <span class="optional">(only if the server has one)</span>
        <input bind:value={serverPassword} type="password" autocomplete="off" />
      </label>
      {#if apiError ?? error}<p class="error" role="alert">{apiError ?? error}</p>{/if}
      {#if mode === "create"}
        <p class="hint">An account keeps your name: nobody else can use it.</p>
      {/if}
      <button class="primary" type="submit" disabled={busy}>
        {mode === "guest" ? "Join as guest" : mode === "login" ? "Log in" : "Create account"}
      </button>
    </form>
  {/if}
</div>

<style>
  .screen {
    display: grid;
    place-items: center;
    height: 100%;
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
  }
  .primary {
    padding: 12px;
    border-radius: 4px;
    background: var(--accent-strong);
    color: white;
    font-weight: 600;
  }
  .link {
    color: var(--text-muted);
    font-size: 14px;
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
