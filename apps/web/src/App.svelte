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
    isKallisti,
    kallistiOrigin,
    kallistiSocket,
    parseAddress,
    trustServer,
    type CertificateProblem,
  } from "./lib/desktop";
  import { httpBase } from "./lib/textChannels";
  import { safeGet, safeSet } from "./lib/storage";
  import { connectionError, proxyUrl } from "./lib/status";
  import type { Place } from "./lib/place";
  import { busyName, loggedOut, reconnectDelay, shouldReconnect } from "./lib/reconnect";
  import { untrack } from "svelte";
  import { checkForUpdate } from "./lib/update";
  import { voiceSupported } from "./lib/voice/support";
  import { settings } from "./lib/settings.svelte";
  import { themeOf } from "./lib/settings";

  // The colour theme (see app.css), also before the login.
  const systemDark = matchMedia("(prefers-color-scheme: dark)");
  let isSystemDark = $state(systemDark.matches);
  $effect(() => {
    const update = () => (isSystemDark = systemDark.matches);
    systemDark.addEventListener("change", update);
    return () => systemDark.removeEventListener("change", update);
  });
  $effect(() => {
    const theme = themeOf(settings.value.theme, isSystemDark);
    document.documentElement.dataset.theme = theme;
    const rail = getComputedStyle(document.documentElement).getPropertyValue("--bg-rail").trim();
    document.querySelector('meta[name="theme-color"]')?.setAttribute("content", rail);
  });

  const PROXY_URL = import.meta.env.VITE_PROXY_URL ?? proxyUrl(location);

  let connection = $state<Connection | null>(null);
  // The account API lives on the Kallisti server (same host as the bridge).
  // The desktop app's server is chosen on the login screen (see below).
  let api = $state.raw(createAccountsApi(httpBase(PROXY_URL)));
  let account = $state<Account | null>(null);
  // Only if the user has not acted yet: a late answer must not undo a login.
  // The desktop app asks its server when one is chosen (see chooseServer).
  if (!isDesktop())
    void untrack(() => api)
      .me()
      .then((acc) => {
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
  // The session ended on the server (a logout or password change on
  // another device): back to the login screen, as after a logout.
  $effect(() => {
    if (connection?.closed && loggedOut(connection.closeReason))
      untrack(() => {
        account = null;
        leave("You were logged out.");
      });
  });
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

  // The desktop app: a Kallisti server works as in the web app (with a
  // session token instead of the cookie); any other address through the
  // app's local Mumble bridge. The web app: through its Kallisti server.
  const desktop = isDesktop();
  let desktopServer = $state(safeGet(localStorage, "server") ?? "");
  let desktopKind = $state<"kallisti" | "mumble" | null>(null);
  let kallisti = $state<string | null>(null); // the Kallisti server's origin
  const sessionStore = (origin: string) => {
    const key = `session:${origin}`;
    return { get: () => safeGet(localStorage, key) || null, set: (token: string | null) => void safeSet(localStorage, key, token ?? "") };
  };
  const chooseServer = async (address: string) => {
    loginNote = null;
    const origin = kallistiOrigin(address);
    const found = origin !== null && (await isKallisti(origin));
    desktopServer = address;
    safeSet(localStorage, "server", address);
    kallisti = found ? origin : null;
    if (found) {
      api = createAccountsApi(origin, fetch, sessionStore(origin));
      account = await api.me();
    }
    desktopKind = found ? "kallisti" : "mumble";
  };
  const changeServer = () => {
    desktopKind = null;
    kallisti = null;
    account = null;
  };
  // A server from last time: looked at again at once.
  untrack(() => desktop && desktopServer && void chooseServer(desktopServer));
  const urlFor = async (credentials: Credentials): Promise<{ url: string; protocols?: string[] }> => {
    if (!desktop) return { url: PROXY_URL };
    if (kallisti) return kallistiSocket(kallisti, sessionStore(kallisti).get());
    const server = parseAddress(credentials.server ?? "");
    if (!server) throw new Error("Enter a server address, for example voice.example.org or voice.example.org:64738");
    return { url: bridgeUrl(await getBridge(), server) };
  };

  // A new version on the server (web app only: the desktop app carries its
  // own): checked now, every 15 minutes, when the tab shows again, and
  // after each (re)connect, as a deploy restarts the server. No automatic
  // reload: it would end voice.
  let updateReady = $state(false);
  let updateClosed = $state(false);
  const checkUpdate = async () => {
    if (!desktop && !updateReady && document.visibilityState === "visible") updateReady = await checkForUpdate(location.origin);
  };
  $effect(() => {
    void checkUpdate();
    const timer = setInterval(checkUpdate, 15 * 60_000);
    const onShow = () => void checkUpdate();
    document.addEventListener("visibilitychange", onShow);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onShow);
    };
  });
  $effect(() => {
    if (connection?.state.status === "synced") untrack(() => void checkUpdate());
  });

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
      const { url, protocols } = await urlFor(credentials);
      if (mine !== starts) return;
      connection = connect(url, credentials, audio, protocols);
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
      const { url, protocols } = await urlFor(lastCredentials);
      if (mine !== generation || !lost) return;
      const next = connect(url, lastCredentials, audio, protocols);
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
        // Refused now (for example banned meanwhile), logged out, or a
        // certificate question: show it; else try again.
        const refused = next.state.status === "rejected" || next.state.status === "kicked";
        if (refused || loggedOut(next.closeReason) || certificateProblem(next.closeReason)) connection = next;
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
    desktop={desktop ? { server: desktopServer, kind: desktopKind, onServer: chooseServer, onChangeServer: changeServer } : null}
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

{#if updateReady && !updateClosed}
  <div class="update" role="status">
    A new version of Kallisti is available.
    <button type="button" onclick={() => location.reload()}>Reload</button>
    <button type="button" class="close" aria-label="Close" onclick={() => (updateClosed = true)}>×</button>
  </div>
{/if}

<style>
  .update {
    position: fixed;
    top: 8px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 60;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 8px 8px 14px;
    border-radius: 8px;
    background: var(--bg-menu);
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.4);
    font-size: 14px;
    white-space: nowrap;
  }
  .update button {
    padding: 4px 12px;
    border-radius: 4px;
    background: var(--accent-strong);
    color: #fff;
    font-weight: 600;
  }
  .update .close {
    padding: 0 6px;
    background: none;
    color: var(--text-muted);
    font-size: 18px;
  }
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
