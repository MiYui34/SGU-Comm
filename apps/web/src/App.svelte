<script lang="ts">
  import Client from "./components/Client.svelte";
  import Login from "./components/Login.svelte";
  import { connect, type Connection, type Credentials } from "./lib/connection.svelte";
  import { connectionError, proxyUrl } from "./lib/status";
  import { voiceSupported } from "./lib/voice/support";

  const PROXY_URL = import.meta.env.VITE_PROXY_URL ?? proxyUrl(location);

  let connection = $state<Connection | null>(null);

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
  <Login {error} onConnect={start} />
{:else if connection.state.status === "synced"}
  <Client {connection} />
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
