<script lang="ts">
  import Client from "./components/Client.svelte";
  import Login from "./components/Login.svelte";
  import { createAccountsApi, type Account, type AccountActions } from "./lib/accounts";
  import { connect, type Connection, type Credentials } from "./lib/connection.svelte";
  import {
    bridgeUrl,
    certificateProblem,
    getBridge,
    isDesktop,
    parseAddress,
    trustServer,
    type CertificateProblem,
  } from "./lib/desktop";
  import { httpBase } from "./lib/textChannels";
  import { connectionError, proxyUrl } from "./lib/status";
  import type { Place } from "./lib/place";
  import { busyName, reconnectDelay, shouldReconnect } from "./lib/reconnect";
  import { untrack } from "svelte";
  import { voiceSupported } from "./lib/voice/support";

  const PROXY_URL = import.meta.env.VITE_PROXY_URL ?? proxyUrl(location);

  let connection = $state<Connection | null>(null);
  // The account API lives on the Kallisti server (same host as the bridge).
  const api = createAccountsApi(httpBase(PROXY_URL));
  let account = $state<Account | null>(null);
  // Only if the user has not acted yet: a late answer must not undo a login.
  // The desktop app has no Kallisti server, so no accounts.
  if (!isDesktop())
    void api.me().then((acc) => {
      if (!connection && account === null) account = acc;
    });
  // An account connection failed (for example an expired session): ask the
  // server again, so the login form comes back instead of a dead end.
  $effect(() => {
    if (error && connection?.account) void api.me().then((acc) => (account = acc));
  });

  // A lost connection comes back by itself (see shouldReconnect): until
  // then the old one stays on screen, with a note.
  const lost = $derived(
    !!connection &&
      shouldReconnect({
        status: connection.state.status,
        closed: connection.closed,
        closeReason: connection.closeReason,
        wasSynced: connection.wasSynced,
      }),
  );
  const error = $derived(connection && !lost ? connectionError(connection.state, connection.closed) : null);

  // Rejected or kicked: do not wait for the server to close the socket.
  $effect(() => {
    if (error) connection?.close();
  });

  // Shown on the login screen after leaving (for example a failed logout).
  let loginNote = $state<string | null>(null);
  const leave = (note: string | null) => {
    loginNote = note;
    stopReconnecting();
    connection?.close();
    connection = null;
    place = null;
    void audio?.close();
    audio = null;
  };
  const accountActions: AccountActions = {
    get temporary() {
      return account?.temporary ?? false;
    },
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
    changePassword: async (old, next) => {
      await api.changePassword(old, next);
      if (account?.temporary) account = { ...account, temporary: false }; // kept now
    },
  };

  // The desktop app connects through its local bridge to the server the
  // user typed; the web app through the Kallisti server.
  const desktop = isDesktop();
  const urlFor = async (credentials: Credentials) => {
    if (!desktop) return PROXY_URL;
    const server = parseAddress(credentials.server ?? "");
    if (!server) throw new Error("Enter a server address, for example voice.example.org or voice.example.org:64738");
    return bridgeUrl(await getBridge(), server);
  };

  // Each start gets a number: a slower earlier start must not win.
  let starts = 0;
  // Desktop: the server's certificate is new or changed. The user decides.
  let lastCredentials: Credentials | null = null;
  const certificate = $derived.by((): (CertificateProblem & { server: string }) | null => {
    const problem = connection?.closed ? certificateProblem(connection.closeReason) : null;
    return problem && lastCredentials ? { ...problem, server: lastCredentials.server ?? "" } : null;
  });
  const trustAndConnect = async () => {
    const server = parseAddress(certificate?.server ?? "");
    if (!certificate || !server || !lastCredentials) return;
    const credentials = lastCredentials;
    try {
      await trustServer(server, certificate.fingerprint);
    } catch (err) {
      connection = null;
      loginNote = `Could not trust the certificate: ${err instanceof Error ? err.message : String(err)}`;
      return;
    }
    await start(credentials);
  };

  // The audio context lives as long as the user stays: reconnects use it
  // again (a new one would need another click).
  let audio: AudioContext | null = null;
  const start = async (credentials: Credentials) => {
    lastCredentials = credentials;
    const mine = ++starts;
    loginNote = null;
    stopReconnecting();
    place = null;
    connection?.close();
    void audio?.close();
    // Created here: browsers allow audio only after a user gesture.
    audio = voiceSupported() ? new AudioContext({ sampleRate: 48_000, latencyHint: "interactive" }) : null;
    try {
      const url = await urlFor(credentials);
      if (mine !== starts) return;
      connection = connect(url, credentials, audio);
    } catch (err) {
      loginNote = err instanceof Error ? err.message : String(err);
    }
  };

  // Reconnecting: one attempt at a time, with growing waits; at once when
  // the tab shows again or the network comes back. A generation number
  // stops attempts that started before a logout or a new login.
  const ATTEMPT_TIMEOUT_MS = 15_000;
  const BUSY_NAME_TRIES = 5;
  let place = $state<Place | null>(null);
  let attempt: Connection | null = $state(null);
  let attempts = 0;
  let busyTries = 0;
  let starting = false;
  let generation = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const stopReconnecting = () => {
    generation++;
    clearTimeout(timer);
    timer = undefined;
    attempt?.close();
    attempt = null;
    attempts = busyTries = 0;
  };
  const tryNow = async () => {
    clearTimeout(timer);
    timer = undefined;
    if (attempt || starting || !lastCredentials || !lost) return;
    starting = true;
    const mine = generation;
    try {
      const url = await urlFor(lastCredentials);
      if (mine !== generation || !lost) return;
      const next = connect(url, lastCredentials, audio);
      attempt = next;
      // A connection that never gets anywhere: give up on it, try again.
      setTimeout(() => {
        if (attempt === next && next.state.status !== "synced") next.close();
      }, ATTEMPT_TIMEOUT_MS);
    } catch {
      schedule();
    } finally {
      starting = false;
    }
  };
  const schedule = () => {
    if (timer === undefined) timer = setTimeout(tryNow, reconnectDelay(attempts++));
  };
  $effect(() => {
    if (lost && !attempt) untrack(schedule);
  });
  $effect(() => {
    const next = attempt;
    if (!next) return;
    if (next.state.status === "synced") {
      untrack(() => {
        connection = next; // the client comes back where it was (place)
        attempt = null;
        attempts = busyTries = 0;
      });
    } else if (next.closed || next.state.status === "rejected" || next.state.status === "kicked") {
      untrack(() => {
        attempt = null;
        next.close();
        // Our old session may still hold the name: try a few more times.
        if (busyName(next.state) && ++busyTries < BUSY_NAME_TRIES) return schedule();
        // Refused now (for example banned meanwhile), or a certificate
        // question: show it; else try again.
        const refused = next.state.status === "rejected" || next.state.status === "kicked";
        if (refused || certificateProblem(next.closeReason)) connection = next;
        else schedule();
      });
    }
  });
  const wake = () => {
    if (lost && document.visibilityState === "visible") void tryNow();
  };
</script>

<svelte:window ononline={wake} />
<svelte:document onvisibilitychange={wake} />

{#if !connection || error}
  <Login
    error={certificate ? null : (error ?? loginNote)}
    {certificate}
    onTrust={trustAndConnect}
    onCancelTrust={() => leave(null)}
    {api}
    {account}
    {desktop}
    onAccountChange={(acc) => (account = acc)}
    onConnect={start}
  />
{:else if connection.state.status === "synced"}
  {#if lost}
    <div class="reconnecting" role="status">
      Connection lost. Reconnecting…
      <button type="button" onclick={() => void tryNow()}>Retry now</button>
    </div>
  {/if}
  <!-- While lost, the old view stays readable but takes no input (it would be lost). -->
  <div class="client" inert={lost}>
    {#key connection}
      <Client {connection} account={account ? accountActions : undefined} resume={place} onPlace={(p) => (place = p)} />
    {/key}
  </div>
{:else}
  <div class="connecting">Connecting…</div>
{/if}

<style>
  .reconnecting {
    position: fixed;
    top: 8px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 50;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 6px 8px 6px 14px;
    border: 1px solid var(--danger);
    border-radius: 6px;
    background: var(--bg-rail);
    color: var(--text);
    font-size: 13px;
    font-weight: 600;
    box-shadow: 0 2px 8px rgb(0 0 0 / 0.3);
  }
  .reconnecting button {
    padding: 4px 10px;
    border-radius: 4px;
    background: var(--bg-active);
    color: var(--text);
  }
  .client {
    display: contents;
  }
  .connecting {
    display: grid;
    place-items: center;
    height: 100%;
    color: var(--text-muted);
  }
</style>
