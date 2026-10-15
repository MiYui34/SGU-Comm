# Devlog

## 2026-10-14: Project start

Goal: a Mumble client with a Discord-like UI. Web first, desktop (Tauri) later.

Architecture for the web version:

- Browsers cannot open raw TCP/TLS or UDP sockets. A small proxy
  (`packages/proxy`) bridges a WebSocket to the TLS control channel of one
  fixed Mumble server. It is not an open proxy.
- The browser speaks the Mumble protocol itself (`packages/protocol`). The
  proxy only moves bytes. Because of this, the same protocol code can later
  run in a Tauri shell with a native TCP socket.
- Voice will go through UDPTunnel messages on the TCP channel, with Opus
  encode/decode by WebCodecs. This costs some latency against real UDP, but
  keeps the proxy simple. A WebRTC bridge is an option for later.
- `Mumble.proto` comes from mumble-voip/mumble `master`. The message order in
  the file is the same as the TCP message type IDs (0..26).

## Proxy notes

- `ws` needs an `error` listener on every server-side socket. Without it, one
  malformed frame (for example an unmasked client frame) crashes the process.
- All proxy users reach Mumble from the proxy IP. Murmur's autoban (10 failed
  attempts in 120 s by default) can ban the proxy and lock everyone out.
  Mitigations: a connection cap now, per-IP limits and a server-side autoban
  exception later.
- TLS ends at the proxy, so a browser cannot present its own Mumble client
  certificate. Users are unregistered for now. Registration needs per-user
  certificates stored at the proxy, or a different identity scheme.

## Web client notes

- The Mumble server does not echo your own text messages. The client adds
  them locally (`addOwnMessage`).
- Murmur kicks an older unregistered session with the same name from the
  same IP ("You connected to the server from another device"). Behind the
  proxy, all users share one IP, so this needs a look once there are
  real users.
- Stale users after a page reload were an artifact of agent-browser's CDP
  reload, which keeps the old socket open. A real in-page navigation closes
  the socket at once. In the worst case (dead peer), Murmur drops the user
  after 30 s without pings, and the proxy then closes both sides.
- Vite must not pre-bundle the workspace protocol package
  (`optimizeDeps.exclude`), or protocol edits do not reach the dev server.
- svelte-check needs the TypeScript 6 API. The web package has
  `typescript@6` next to the native `typescript@7`.

## Architecture direction (2026-10-14)

A deployment will be Murmur + our server + the web app. The proxy grows into
that server: it keeps the dumb byte bridge, and adds a separate API for text
channels, history, uploads and identity (TLS ends there, so it can hold
per-user certificates). Desktop Mumble users keep voice and in-room chat
only. The protocol and voice code must not depend on the sidecar.

Server language: Rust or Go are preferred. Decide when sidecar work starts.

## Voice notes

- Murmur 1.5 sends protobuf audio (`MumbleUDP.proto`, first byte 0 = Audio)
  to clients that announce 1.5, also inside UDPTunnel frames. Older servers
  use the legacy varint format. The client picks the format from the
  server's `Version` message.
- Voice over TCP is never lost or reordered, only late. Playback schedules
  each decoded chunk right after the previous one (in whole samples), and
  drops chunks when the queue is longer than 300 ms.
- The server does not echo our own voice. The speaking ring for ourselves
  comes from the capture side.
- The last frame of a transmission is sent with the terminator flag. The
  encoder keeps the order, so a FIFO of flags maps outputs to packets.
- Vite inlines the small capture worklet as a `data:` URL. A strict CSP must
  allow `data:` for worklets, or the build must emit it as a file.
- Testing: `agent-browser --args` breaks the browser launch here, so Chrome's
  fake media flags are not available. Instead, inject a fake microphone
  before connecting: replace `navigator.mediaDevices.getUserMedia` with a
  stream from an oscillator. `scripts/voicebot.ts` plays an Opus file into
  the server for playback tests.
- Do not call `encoder.flush()` after the terminator frame. In Chrome it
  emits an extra packet for the Opus lookahead, after the terminator and with
  a repeated frame number. Chrome's encoder gives one output per 20 ms input,
  so no packet is held back. Firefox and Safari are not checked yet.
- In this sandbox, `navigator.mediaDevices.enumerateDevices()` and a
  `devicechange` listener crash the headless Chrome tab (it lands on
  `about:blank`). The fake microphone script for tests must stub both.
- Svelte 5: an effect that reads `prefs.x`, where `prefs` is a derived
  object that is replaced on every change, re-runs on any settings change.
  This reopened the microphone on every slider step. Fix: derive the
  primitive (`$derived(prefs.inputDeviceId)`), which only notifies when the
  value changes.

## Server language: Go (2026-10-14)

The server (bridge now, sidecar later) moves to Go, in `server/`. Reasons:
fast to write network servers in, natural backpressure with blocking I/O,
and `pion` is a strong option if a WebRTC voice bridge is needed later.
WebSocket library: `github.com/coder/websocket`.
- Go bridge: `coder/websocket`'s `Close` waits up to 5 s for the peer's close
  frame. Shutdown therefore closes connections from a hook, while the
  connection's own reads keep running to receive that frame. Cancelling the
  read context instead drops the connection without a close frame.
- Bun replaces the `ws` package with its own WebSocket. It reports close code
  1000 where Node's `ws` shows the real code (1001). Use Node to check codes.

## Name: Kallisti (2026-10-14)

The product is called Kallisti (kallisti.chat). Packages use the `@kallisti/*`
scope, the Go module is `kallisti.chat/server`, and the client announces
itself as "Kallisti" to Mumble servers. The Gitea repository and the local
folder keep the old name "mumble-discord".

## Text channels (sidecar)

- The server now reads the Murmur-to-browser stream passively (it never
  changes it) to learn each connection's session and name from ServerSync
  and UserState. This replaces the rule "the server never parses the
  stream". The browser cannot fake this identity, but an unregistered name
  is only unique while its owner is online. Accounts will fix that.
- The JSON API uses WebSocket text frames on the same connection. Requests
  are rate limited (5/s, burst 10) and capped at 16 KB.
- SQLite runs with WAL and `synchronous=NORMAL`: a message insert happens in
  the sender's read loop, and a full sync on every insert would delay their
  voice.
- Playback limiter: Chrome's DynamicsCompressorNode adds make-up gain
  (1.14 at threshold -2 dB, ratio 20, knee 0), which made all quiet audio
  louder. A fixed gain of 1/1.14 after it cancels that. Measured with an
  OfflineAudioContext: full-scale tone at 200 % peaks at 0.85, levels below
  the threshold pass unchanged.

## Deployment (2026-10-14)

- One origin: the Go server serves the built web app, the bridge (WebSocket
  upgrades, the client uses `/ws`), uploads and files. The web app has a
  strict CSP; the capture worklet is therefore emitted as a file, not inlined
  as a data: URL.
- Tested with `podman compose` and `DOMAIN=localhost` (Caddy's internal CA):
  the app loads over HTTPS, and a wss client gets Murmur's handshake and the
  text channel list through Caddy. Not yet tested with a public domain.
- Bun's WebSocket ignores `rejectUnauthorized` (close code 1015 with a local
  CA). Use Node's `ws` for TLS checks against self-signed certificates.

## Desktop app (2026-10-14)

- Tauri v2. A local bridge in Rust (127.0.0.1, random port and secret, Origin
  check) lets the unchanged web client talk TLS to any Mumble server.
- Server certificates: trust on first use. The bridge only lets the page pin
  a fingerprint that it saw itself on a real connection.
- Tested: Rust integration tests (TLS echo, real Murmur) and the web client's
  desktop mode in a browser with a stubbed `__TAURI_INTERNALS__` (agent-browser
  `--init-script`). The app window itself could not be opened in the
  development sandbox; `tauri build` produces Kallisti.app.

### Native UDP voice

- OCB2-AES128 and CryptState are ports of Mumble's code. OCB2 has a
  deliberate countermeasure (XEX*) that changes an all-zero block, so test
  data must not be all zeros.
- The bridge reads Version and CryptSetup from the server's stream, pings
  over UDP, and sends voice over UDP while a good UDP packet came in the last
  12 s. Only whole frames go to the browser, so UDP voice never lands inside
  a TCP frame.
- Murmur sends voice to a client over UDP only after that client sent voice
  (not a ping) over UDP, and goes back to TCP when a UDPTunnel arrives over
  TCP. So when UDP stops, the bridge sends a ping in a UDPTunnel frame; a
  silent user would otherwise hear nothing.
- UserStats `udp_packets` is what the client reports about itself. Murmur's
  own count of decrypted UDP packets is `from_client.good`.
- Murmur's autoban blocks an IP after 10 connects in 120 s. The dev
  container runs with `MUMBLE_CONFIG_AUTOBANATTEMPTS=0` for the tests.

## Built-in Mumble server spike (2026-10-14)

- Grumble (Go Murmur) was checked briefly: little activity since 2023,
  announces 1.2.5, legacy UDP format only. Not a base.
- A minimal server in Go (`internal/mumbled`) passes the same bridge tests as
  Murmur, and the web client works through it with voice and text.
- Murmur (Server.cpp) sets a client's UDP flag only for UDP audio, not for
  pings, and clears it for a UDPTunnel over TCP. It requests a nonce resync
  when no packet decrypted for 5 s, at most every 5 s.
- Closing a `tls.Conn` waits for its writer (close_notify needs the write
  lock), so the server never closes a connection while it holds a lock.

## Built-in Mumble server (2026-10-14)

- `server/internal/mumbled` replaces Murmur: channels, accounts, Murmur's
  ACL model, moderation, whispers and listeners, Mumble clients before 1.5,
  comments and avatars, limits. The Kallisti server runs it by default; web
  clients join in-process (net.Pipe from the WebSocket bridge, with the real
  client IP), Kallisti accounts are Mumble registrations by their
  certificate hash. One container, no Murmur.
- Structure: `State` (channels, users, ACL evaluation, receivers) has no
  IO; handlers check permissions and call it under one lock. A change
  counter invalidates cached voice receivers, so no ACL evaluation runs per
  voice packet.
- The official client in podman (Xvfb, `packages/devtools/mumble-client`)
  tests every step: Mumble 1.5 from Debian trixie, 1.3.4 from bookworm.
  First-run dialogs: 1.5 reads a JSON settings file (`settings_version`
  and categories), 1.3 an ini file; certificates as base64 PKCS#12.
  xdotool drives menus (the ACL editor, "View Comment", "Listen To
  Channel"); there is no window manager, so tooltips do not show.
- Protocol details that cost time:
  - proto2 defaults: `ACL.ChanACL.inherited` and `ChanGroup.inherited`
    default to true, so a missing field means "inherited"; always send it,
    and do not filter on it when reading.
  - `BanList.BanEntry.address` and `mask` are required fields.
  - Legacy UDP positions are floats in native (little-endian) order.
  - A field in a UserState is a change for clients: send only set flags,
    or the official client logs "unmuted by the server" for new users.
  - Murmur switches a client to UDP voice only after UDP audio from it,
    not after a ping, and back to TCP on a UDPTunnel over TCP.
- Murmur's source (Messages.cpp, Server.cpp, ACL.cpp, Group.cpp) is the
  reference for every rule; reviews compared against it each step.

## 2026-10-15: Vite hides the CSP

Avatars (data: URLs) worked in every browser test and broke in
production: the Go server sends a CSP (`img-src 'self' blob:`), Vite
sends none. Browser tests of anything that loads resources (images,
media, workers, connections) must also run against the built app on the
Go server (`pnpm --filter @kallisti/web build`, then the server with
`WEB_DIR=apps/web/dist`).

## 2026-10-15: Chat features (Markdown, reactions, typing, previews)

- Murmur reads text messages longer than `textmessagelength` as XML (to
  check the length without images) and refuses invalid ones: `<br>` must
  be `<br/>` in messages with images (found with the Jukebox covers).
- Markdown runs on escaped text; code and links are set aside first (NUL
  placeholders, NUL stripped from input), so nothing inside them is
  formatted. Mentions for highlights, badges and notifications all use
  the written text, not the Markdown HTML.
- Reactions: emoji have several forms that look the same (with or without
  U+FE0F, joiners at the end). The server normalizes (FE0F after the older
  BMP symbols, nowhere else) and allows only symbols plus what sequences
  add (skin tones, FE0F, tags, joiners between parts). The picker uses the
  same form.
- Link previews: the server fetches (readers' IPs stay private). The
  dialer's Control function checks the resolved address of every
  connection (redirects, DNS rebinding), with keep-alive off (idle sockets
  to hostile hosts), a queue limit, one deadline for page and image, and a
  byte limit for cached images. Clients ask per message over the sidecar
  (read permission applies), not per URL over HTTP.

## 2026-10-15: Guest accounts and user ids

- Web guests get temporary accounts (cookie, no password, a Mumble
  registration on the built-in server); a password keeps one, a day without
  a visit removes it. Only with the built-in server: Kallisti cannot remove
  registrations on an external Murmur.
- Once accounts are deleted, names come back: everything that was keyed by
  name (message ownership, read marks, reactions) moved to the Mumble user
  id. SQLite reuses the highest INTEGER PRIMARY KEY after a delete, so the
  built-in server now keeps the highest user id ever given (mumble_meta)
  and never gives an id twice.
- The migration maps names to ids with the built-in server's mumble_users
  (same database, names.Key). Tested on a copy of the live database
  before the deploy (and a second start, which must change nothing).
- Cleanup and registration of web accounts take turns on one lock; the
  server does not register a web account whose account is gone, and does
  not remove the registration of a user who is online.

## 2026-10-15: RNNoise

- RNNoise is the Jitsi build (@jitsi/rnnoise-wasm, 112 KB .wasm) with our
  own 30-line wrapper in the capture worklet: the module only imports
  memory grow and memcpy, so the 1.9 MB Emscripten loaders are not needed.
- Chrome does not post a compiled WebAssembly.Module to an AudioWorklet
  (messageerror on the worklet side, nothing on the main thread). The main
  thread posts the bytes; the worklet compiles them itself.
- The CSP needs script-src 'wasm-unsafe-eval' (only WebAssembly, not
  eval). Found only with the built app: Vite sends no CSP.
- An AudioContext in a headless test tab only runs after a click; worklet
  experiments must come after one.
