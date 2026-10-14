<script lang="ts">
  import Client from "./components/Client.svelte";
  import Login from "./components/Login.svelte";
  import { createAccountsApi } from "./lib/accounts";
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

  const start = (credentials: Credentials) => {
    connection?.close();
    // Created here: browsers allow audio only after a user gesture.
    const audio = voiceSupported()
      ? new AudioContext({ sampleRate: 48_000, latencyHint: "interactive" })
      : null;
    connection = connect(PROXY_URL, credentials, audio);
  };
</script>

{#if !connection || error}
  <Login {error} {api} {account} onAccountChange={(name) => (account = name)} onConnect={start} />
{:else if connection.state.status === "synced"}
  <Client
    {connection}
    onLogout={account
      ? async () => {
          await api.logout();
          account = null;
          connection?.close();
          connection = null;
        }
      : undefined}
  />
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
