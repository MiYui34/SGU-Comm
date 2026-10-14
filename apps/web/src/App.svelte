<script lang="ts">
  import Client from "./components/Client.svelte";
  import Login from "./components/Login.svelte";
  import { createAccountsApi, type AccountActions } from "./lib/accounts";
  import { connect, type Connection, type Credentials } from "./lib/connection.svelte";
  import { httpBase } from "./lib/textChannels";
  import { connectionError, proxyUrl } from "./lib/status";
  import { voiceSupported } from "./lib/voice/support";

  const PROXY_URL = import.meta.env.VITE_PROXY_URL ?? proxyUrl(location);

  let connection = $state<Connection | null>(null);
  // The account API lives on the Kallisti server (same host as the bridge).
  const api = createAccountsApi(httpBase(PROXY_URL));
  let account = $state<string | null>(null);
  // Only if the user has not acted yet: a late answer must not undo a login.
  void api.me().then((name) => {
    if (!connection && account === null) account = name;
  });
  // An account connection failed (for example an expired session): ask the
  // server again, so the login form comes back instead of a dead end.
  $effect(() => {
    if (error && connection?.account) void api.me().then((name) => (account = name));
  });

  const error = $derived(connection && connectionError(connection.state, connection.closed));

  // Rejected or kicked: do not wait for the server to close the socket.
  $effect(() => {
    if (error) connection?.close();
  });

  // Shown on the login screen after leaving (for example a failed logout).
  let loginNote = $state<string | null>(null);
  const leave = (note: string | null) => {
    loginNote = note;
    connection?.close();
    connection = null;
  };
  const accountActions: AccountActions = {
    logout: async () => {
      const ok = await api.logout();
      if (ok) account = null;
      leave(ok ? null : "Logging out did not finish: you may still be logged in.");
    },
    logoutAll: async () => {
      await api.logoutAll();
      account = null;
      leave("Logged out on all devices.");
    },
    changePassword: api.changePassword,
  };

  const start = (credentials: Credentials) => {
    loginNote = null;
    connection?.close();
    // Created here: browsers allow audio only after a user gesture.
    const audio = voiceSupported()
      ? new AudioContext({ sampleRate: 48_000, latencyHint: "interactive" })
      : null;
    connection = connect(PROXY_URL, credentials, audio);
  };
</script>

{#if !connection || error}
  <Login error={error ?? loginNote} {api} {account} onAccountChange={(name) => (account = name)} onConnect={start} />
{:else if connection.state.status === "synced"}
  <Client {connection} account={account ? accountActions : undefined} />
{:else}
  <div class="connecting">Connecting…</div>
{/if}

<style>
  .connecting {
    display: grid;
    place-items: center;
    height: 100%;
    color: var(--text-muted);
  }
</style>
