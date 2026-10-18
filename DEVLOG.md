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
- Encoder outputs are matched to inputs in order, not by timestamp
  (2026-10-16). Firefox shifts output timestamps by the Opus pre-skip
  (input 20000 µs, output 13500 µs) and holds the last packet back until
  `flush()`. So the terminator frame is flushed, and outputs after the last
  input are dropped: both Chrome and Firefox emit one extra packet on flush.
- An inactive worklet input has no channels; Firefox gives that for a
  digitally silent source. The capture worklet treats it as silence, so the
  voice detection still ends the spurt.
- `server/cmd/voicecheck` joins a channel and checks frame numbers and
  terminators per speaker. Firefox can be tested in the Playwright image with
  PulseAudio added (without a sound device its AudioContext stays
  suspended), with a fake microphone from an oscillator whose gain is
  switched on and off. In Lockdown Mode Safari has no AudioWorkletNode.
- CI (2026-10-16): `.github/workflows/voice.yml` runs `tests/voice/run.sh`
  in Chrome and Firefox (Linux, PulseAudio for a sound device) and Safari
  26.6 (macos-26 runner, safaridriver), driven by plain WebDriver. All pass.
  With the encoder fix reverted, Firefox fails (no terminators); Safari
  keeps input timestamps, as Chrome does.
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
- DeepFilterNet3 comes from @workadventure/noise-suppression (only its DFN
  part ends up in dist: worklet, 14.6 MB engine, 8 MB model). It passes the
  compiled module in processorOptions, which works where port messages
  fail. Its dispose() does not free the filter state, so there is one node
  per AudioContext, kept and connected in or out. The Go server now gzips
  assets (the engine: 4 MB).

## Screen sharing (2026-10-16)

- WebCodecs over a WebSocket of its own (`/api/stream`): the server only
  forwards frames. VP8 first: a Chromium without proprietary codecs claimed
  to decode H.264 (`isConfigSupported` true) but failed on every frame.
- Picture and sound tracks from Chromium's tab capture stamp their frames on
  different clocks. Viewers that synced pictures to the sound clock showed a
  black picture (tests with a canvas and an oscillator shared one clock and
  passed). Both tracks are now put on the page's clock from their first
  frame, and viewers show pictures that seem far ahead at once.
- Hidden tabs (the usual case while sharing another window): frames come from
  MediaStreamTrackProcessor where it exists; a worker timer elsewhere (blob
  workers are blocked by the CSP: the worker is a file of its own).
- Mumble clients hear the stream from a hidden user that is not in the
  server's user list (nobody can message, move or kick it).

## Emoji picker (2026-10-17)

- Data from emojibase (names, keywords, GitHub shortcodes), cut to the
  Twemoji version and written to `public/emoji-data.json` at build time; the
  picker fetches it when it first opens (51 KB gzipped).
- emojibase's sequences are not always fully qualified (`🎓` with a
  selector), so they are not a source for the selector rules. Reactions use
  the fully qualified form from Unicode's `emoji-test.txt`: the selector
  after code points with Emoji_Presentation=No (a list that no longer
  grows), not before a skin tone. That rule reproduces all 3944 fully
  qualified sequences. The old rule (a selector after every older symbol)
  broke text-style emoji with skin tones; stored reactions move to the new
  form at start.
- Go's `unicode` tables are at Unicode 15: the emoji block counts as
  symbols as a whole, or Emoji 16/17 would be rejected.
- `@twemoji/parser` 17.0.2 does not find text-style emoji with skin tones
  (✌️, 🖐️, ☝️ …); 17.0.1 does. 👁️‍🗨️ shows as two images with it.

## Small gaps to Discord (2026-10-17)

From `meta/research/discord-comparison.md`.

- Spoiler attachments are marked by the file name (`SPOILER_` prefix), as
  in Discord: no new field on the server. Links inside `||spoilers||` get no
  preview card (client and server skip them), or the card would show them.
- Blocking is local to the browser, by lower-case name, like the voice
  mute. The server does not know the list, so a channel's unread dot and
  the mention count from before the login still include blocked users.
- `@here` reaches only who is online when it is sent: the server's summary
  at login counts `@everyone`, not `@here`, and the client counts `@here`
  only in live messages. The server stores which one a message has, and
  only for moderators; for others both stay plain text. An edit can take
  the ping away but not add one.
- Message links are `/channels/<chat>/<message>`; the server already sends
  the app for unknown paths. A jump into a chat waits for its first history
  page, or the jump gave up at once (also for search results).
- Search without words: an `OR` in the FTS condition made SQLite scan the
  whole table (0.1 ms became 13 ms at 300k rows), so the query has two forms.
- Chrome needs no service worker to install a web app any more; the
  manifest and icons are enough.

## Discord gaps, second round (2026-10-17)

- Built partly by parallel agents in git worktrees, merged at the end. The
  merges conflicted only where each branch added to the same lists (ready
  event fields, request types, the user menu); resolving meant keeping both.
- Light theme: a plain script in `<head>` (`public/theme.js`, allowed by
  `script-src 'self'`) sets the theme before the CSS paints; the app sets it
  again from the settings. The default stays dark.
- Channel notifications keep the old localStorage key: the parser reads
  the old list of muted ids as mutes "until turned off".
- Custom emoji images are blobs in SQLite, not files in UPLOAD_DIR: the
  cleanup of unattached uploads would delete them. Their ids are never used
  again, so the image URLs can be cached for ever. `:name:` images are added
  after sanitizing, like Twemoji, so message HTML cannot forge them.
- The bio is the Mumble user comment. Long comments come only as a hash;
  the web client fetches them when shown (user menu, settings), not all at
  once like avatars.
- Statuses: "invisible" does not fit Mumble (everyone sees who is
  connected). Away is sent by the client and not stored; the chosen status
  is stored for registered users.
- Time-outs mute in the built-in server and block chat in the sidecar.
  Guests are timed out by session, so reconnecting ends it for them; a ban
  is the tool for guests.

## Desktop app as a Kallisti client (2026-10-18)

- The desktop app's page has its own origin (tauri://localhost, or
  http(s)://tauri.localhost on Windows), so the server's session cookie is
  third-party there (WebKit blocks it). The app asks for the session token
  at login (`X-Kallisti-Session: token`) and sends it as
  `Authorization: Bearer`, and on the WebSocket as a subprotocol
  (`kallisti.session.<token>`; the server answers `kallisti`, as browsers
  require one): browsers cannot set WebSocket headers, and URLs end up in
  logs.
- The app origins are allowed on every server, and the cross-site guard
  skips them only without a cookie: other Tauri apps share the origins, and
  a local web server can pose as http://tauri.localhost.
- The app tells a Kallisti server by its exact answer to /api/me, without
  redirects. A timeout or offline is "cannot reach", not "Mumble server".
- Mumble clients can log in to Kallisti accounts with the account password
  (the built-in server asks the account store; no server password needed).

## Review before v0.9.0 (2026-10-18)

- An external review (GPT-6 Astra) found 30 points; 16 of the 17 bug
  claims were confirmed against the code. The worst: a voice target with
  linked channels kept a removed channel, and the next voice packet
  crashed the server (temporary channels go away by themselves, so any
  registered user could do it). Fixed and deployed at once.
- One ChatPane serves every chat: per-chat state (reply, edit, draft)
  must be reset or kept by chat id by hand; rows are keyed by message id,
  as reactions replace message objects.
- All per-address limits count an IPv6 /64 as one address.
- CI now runs the unit tests and type checks (test.yml).

## Invite-only channels (2026-10-18)

- Behaviour change: the built-in server now hides every channel a user
  may not Traverse, also outside private channels (Murmur shows them).
- One source of truth: a "members" group and ACL on the Mumble channel
  (for text channels, on their mirror). Kallisti checks text access
  through it, so Mumble clients and the web agree.
- Manual check (web): a member sees both channels and writes; a
  non-member sees no channel, message or search hit; a removed member
  loses the channel and its messages at once.

## Make admins (2026-10-18)

- Only SuperUser changes root's ACL through a Mumble client: root holds
  the roles, and an admin could otherwise drop other admins from the
  "admin" group, or the entry that gives the group Write. Admins can still
  read it, and use set-admin/set-moderator.

## Invite links (2026-10-18)

- Links are /invite/<token> paths: the server already answers every
  unknown path with the app, and the address stays through login or guest
  join (no reload), so no token has to be stored in the browser.
- Invites are keyed by the Mumble channel (a voice channel, or a text
  channel's mirror), like private channel access itself. The redeem
  answer names the text channel if there is one.
- A use is counted (one conditional UPDATE) before the member is added:
  no limit is overrun, but a failed add loses the use.
- Links of channels that stop being private are pruned on every access
  change (PrivateChanged); redeem also checks that the channel is still
  private, so pruning is only cleanup.

## Offline members (2026-10-18)

- Guest accounts are Mumble registrations too; only the store knows they
  are temporary (by name). The sidecar filters them, mumbled stays
  unaware of Kallisti accounts.
- The list is sent once with ready, then "member" deltas (add, rename,
  gone). Other lists (roles, private) are resent whole, but registrations
  come with every guest join, so a full resend would be wasteful.
- Bios are not in the list: Mumble comments can be large (HTML with
  inline images). The menu asks for an offline user's stored comment
  ("bio" request) when it opens.
- Stored DMs to offline users already worked (DirectChat needs only the
  user id); the menu just offers "Message". Role changes also worked
  (setRole checks the registration, not a session).

## Offline members: follow-ups (2026-10-18)

- "online" events tell every Kallisti client the session of each member
  that is connected, in any channel, from mumbled's state and not from
  the viewer's UserStates. Clients list members they do not see as online
  without a channel; only the id and session go out (statuses already go
  by session to everyone). Everyone gets the same set, so a viewer can
  infer "online, but somewhere I cannot see", as on Discord.
- Guests in hidden channels stay invisible: unregistered users have no
  id, and guest accounts are left out of the online set like out of the
  member list. Mumble clients see nothing new.
- A web account's first visit registers it before its session joins:
  mumbled now reports the registration after the join (and a Mumble
  client's self-registration after the state change), so the client gets
  "online" before "member" and never lists a new user as offline.
- Known gap: the Mumble UserState/UserRemove and the "online" event take
  different paths (mumbled's event queue), so a user who joins or leaves
  can show for a moment in Online without a channel.
- mumbled allows one session per registered user (another device kicks
  the old one), so a member has one session and one status.
- Making a guest account permanent changes only the store: auth tells the
  hub (OnPermanent), which finds the registration by name (accounts keep
  their registration's name, see identifyAccount).

## Music mode (2026-10-18)

- Stereo needs nothing from the protocol: the channel count is in each
  Opus packet (TOC byte), not in the Mumble header. A mono Opus decoder
  (old Mumble clients) mixes stereo packets down; our playback decodes
  with two channels already. The frame size stays 20 ms, so frame
  numbers do not change.
- WebCodecs' Opus `application` and `signal` members are newer than
  TypeScript's DOM types (typed locally in voice/opus.ts). Browsers
  without them ignore unknown dictionary members, so configure() does
  not fail; those browsers get the default (voip) tuning at 128 kbps.
- Chrome gives two channels only with echo cancellation off; music mode
  turns it off and asks for `channelCount: { ideal: 2 }`. Stereo is
  decided by the track's getSettings().channelCount (Safari may not say:
  then mono).
- The capture worklet sends planar frames (all left, then all right).
  The main thread derives the channel count from the frame length, because
  frames sent before the worklet gets `{ channels }` still have the old
  count. A new count or a music mode switch gets a new encoder at the next
  frame, also in the middle of a talk spurt; the old one is flushed first,
  or Chrome keeps its last packet and the encoder is never closed.
- Server: mumbled counts payload + 32 bytes of headers per packet against
  558 kbps. 128 kbps in 20 ms frames is about 143 kbps with headers;
  Opus VBR peaks stay well under the limit. No server change.

## Mass delete (2026-10-18)

- Direct messages are left out on purpose: staff do not read DMs, so they
  should not delete them unseen either.
- Web guests are registered (temporary accounts), so they go by user id
  like everyone else, offline too. Only unregistered Mumble guests go by
  session, and only for what they wrote in that connection (the sidecar's
  in-memory map, cleared when they leave). So the web client sends the
  delete before a kick or ban: the bridge handles text frames in its read
  loop, so the order holds.
- Known gap: unregistered guests on Mumble apps are not in that map (it
  is filled by web sends only and cleared on the web connection's close),
  so their messages cannot be found. Single deletes by staff still work.
- One "messages-deleted" per chat; a chat with one message still gets the
  old "message-deleted" (the desktop app bundles its web client, older
  builds do not know the new event).
- At most 1000 messages per request (newest first); a bigger spam wave
  needs another request.

## Guests read only (2026-10-18)

- The rule is in mumbled's `State.Permissions`: read-only guests lose
  TextMessage everywhere. So Mumble clients get PermissionDenied
  (TextMessage) for channel, tree and private texts, `MayWrite` (voice
  chats) follows, and the permission flush greys out their text box.
  `isStaff` now reads the ACLs alone (`aclPermissions`), else it would
  call `Permissions` again.
- Guest accounts are registered users: mumbled learns them at login
  (`Config.GuestAccount`, a store lookup) and keeps a flag on the user.
  Setting a password clears it (`Server.AccountPermanent`, next to the
  hub's) without a reconnect.
- The sidecar checks writing requests in one place (`readOnly`, next to
  `timedOut`) through `Voice.ReadOnly`, so mumbled is the only source of
  who is a guest. Upload tokens are refused; a token issued before the
  switch can still upload a file, but the message with it is refused.
- The setting is a row in `mumble_meta`; Murmur servers do not have it.
- Web: the composer gives way to a note with a button. A guest account
  opens the settings to set a password (and writes at once); a guest
  without an account goes back to the login screen.

## Connection info (2026-10-18)

- Ping answers come back with our timestamp, but the server's bridge also
  pings upstream (UnixMilli), and those answers reach the browser too. The
  client keeps its own timestamps (µs of `performance.now()`) and ignores
  the rest. Pings before sync only keep the connection: during the
  handshake the answer waits for the login and looked 20 ms slow.
- Pings now go every 5 s (Mumble's interval), not 15 s, so the 30 s window
  has about six samples.
- Late audio: a chunk that goes on the stream (frame numbers contiguous)
  but finds its queue run dry, or is dropped for a full queue. Our capture
  numbers frames without gaps across talk spurts, so the terminator must
  reset the expectation, else every new spurt counts as late. Decoder
  outputs come in order, so a queue beside the decoder (`queued`) carries
  each chunk's frame time and terminator flag; a bare terminator marks the
  last queued chunk. The frame time comes from the queue, not from the
  output's timestamp, which a decoder may count itself.
- Lost audio (frame number jumps up to 200 ms; longer is taken for a new
  spurt without terminator) only happens over the desktop app's UDP;
  over TCP it stays 0.
- Ping fields as Mumble: `tcp_ping_var` is the variance (clients show its
  square root), `tcp_packets` the samples behind the mean, good/late/lost
  totals since connect over all speakers.
- Levels: good below 100 ms and 2 %, fair below 250 ms and 8 %, else poor
  (the worse of both). The page cannot see whether the desktop bridge
  uses UDP, so it says "UDP or TCP".

## Connection stats (2026-10-18)

- Mumble numbers voice in 10 ms frames, so the frame number of a 20 ms
  packet goes up by 2. The web client takes it from the capture
  timestamp. Over TCP, mumbled counts by frame number: the step is the
  smallest one in a transmission, a gap is lost frames, an older number
  is late. A terminator, or a jump of more than 50 steps, starts a new
  transmission (the web client's numbers jump after a pause). Over TCP,
  "lost" means frames the client never sent, for example a throttled
  tab, not network loss.
- The web bridge sends its own keep-alive Pings (timestamp only)
  upstream, between the browser's. Murmur takes every ping field as it
  is. mumbled keeps only the fields a Ping has, otherwise each bridge
  ping would set the ping times to 0.
- The server cannot measure TCP round trips itself: a Mumble client
  takes every Ping it gets as the answer to its own. For web users the
  bridge's WebSocket ping could measure it, but nothing does that yet.
- UserStats follows Murmur's msgUserStats: version, address and
  certificates for oneself and for Register in root (not with
  stats_only); counters for the same channel; others need Enter (else
  PermissionDenied). Hidden users still get no answer. `from_client`
  is UDP (crypt) plus TCP (frame numbers). The Ping answer stays UDP
  only, because Mumble clients read it as their UDP stats.
- `Server.ConnectionStats()` is for the admin view. Sidecar overflows
  come in through `Config.TextOverflows` (`Hub.Overflows`), so mumbled
  does not import the sidecar.

## Connections view (2026-10-18)

- The window lives in the server, not per poll: every 10 s mumbled keeps
  a snapshot of each connection's counters (the last three). A poll and
  the log line count against the oldest one, so the window is 30 to
  40 s, and several admins polling share it.
- Mumble's crypt (and the server's TCP count) counts a late frame as
  good too; the web client's reports count it apart. The server takes
  the crypt's way for both: late or lost / (good + lost). For web
  clients that overstates a little (8.7 % instead of 8 %). Fewer than
  50 frames in the window give no percentage (one late frame in a quiet
  window looked like 50 %).
- In the crypt a late frame was counted lost first, so lost goes down
  again: within a window lost can shrink. Bad is late plus the change in
  lost (not below 0); only a smaller good means a client that started
  its counters over.
- Ping stays the client's: Mumble clients report the mean since they
  connected, so for them it moves slowly; the web client's is over 30 s.
- Log hysteresis: "connection poor" after 30 s poor, "connection good
  again" after 30 s good; fair resets both timers, so a connection near
  a limit stays quiet. Same levels as the web client's.
- No addresses in the table or the request: admins see them in
  Mumble's user information, a table on the screen shows them too
  easily.
- `sidecar.Connection` has the same fields as `mumbled.Connection`, so
  cmd/server converts it with a plain struct conversion.

## Slow mode (2026-10-18)

- Text channels only. Voice channel settings are Mumble channel state
  (Write in mumbled), and their chats are made lazily in the store; a
  slow mode there would need a second home for the setting. Direct chats
  have none.
- The turn is taken just before the message is stored, after all other
  checks: a refused text or a wrong reply costs no turn. Uploads go
  through the same `send`, so they count, but the file is uploaded
  before the refusal: the cleanup of unused uploads removes it.
- Last posts are in memory (keyed by user id, guests by session) and
  recorded also while slow mode is off: the composer counts from the
  newest own message it has loaded, so turning slow mode on must hold
  back who just posted on the server too. After a restart the server
  lets a post through that the composer still holds back (stricter,
  never looser). Clock skew: the composer counts in server times and
  caps the wait at the slow mode.
- mumbled asks before its time-out and permission checks: a timed out
  Mumble user in a slow channel can get the slow mode refusal first,
  and a refused text still takes the turn.
- Mumble clients: mumbled asks `Config.CheckChannelText` before it
  relays a text, outside its lock (the hub calls back into mumbled for
  the staff check). A refused text reaches nobody; the sender gets a
  PermissionDenied of type Text, which Mumble shows in its log as
  "Slow mode: wait 23 s". Only channels written to directly are asked
  (a tree message is not stored in a mirror, see mumbled textMsg).
- Web and Mumble share one turn per user and channel: a registered user
  waits on the web after writing from Mumble.

## Message rules (2026-10-18)

- One rule set for both issues (new-account limits, word and link
  filter): `internal/msgrules` is pure (rules, matching, `Check`);
  mumbled keeps the rules (JSON in a new `mumble_settings` table:
  `mumble_meta` holds integers), the registration times and the last
  message times, so a limited user has one rate over web and Mumble.
  The sidecar has one hook (`refused`, next to `readOnly`) that calls
  `Voice.CheckMessage`. With Murmur there are no rules.
- Registration time is a new `mumble_users.registered_at` (0 for rows
  from before, so old accounts are never new). A Kallisti account gets
  its Mumble registration on its first visit, so an account made before
  the update and first used after it counts as new once. A guest
  account that gets a password keeps the time of its first visit.
- Words: NFKC, without format characters (zero-width spaces) and
  fillers that count as letters (Hangul fillers, Arabic tatweel), Unicode
  case folding, without the accents of Latin, Greek and Cyrillic letters
  (not of others: a Japanese dakuten makes another letter). Then whole
  words (letters, digits, marks; `_` and `.` separate, so "spam" also
  blocks "spam.example"), phrases as word sequences. Han, kana, Thai,
  Lao, Khmer and Myanmar have no spaces between words: there an entry
  matches anywhere. Entries are words of letters and digits (with ' - . _
  between): "c++" would be the word "c" and block every "c". Homoglyphs
  (Cyrillic а for Latin a) and spaced letters ("s p a m") are not
  caught.
- Domains: every host-like token in the text (also bare "junk.example",
  markdown links, `user@host`, the ideographic full stop, percent-encoded
  hosts, which browsers decode) in its IDNA ASCII form, against each
  entry and its subdomains; a host that is no valid IDNA is compared as
  it is. Mumble texts count by their plain text: mumbled strips the tags
  (link targets too) before it sends a text on.
- Links for new accounts are what clients make links of: a scheme
  (`x://`) or `www.`. A bare domain is no link. @everyone/@here only
  pings for staff anyway; for limited users it is refused as noise on
  the web (Mumble texts never ping, and are not checked for it).
- The wait of limited users starts with a message the rules let
  through, also if the send fails later (channel rights, length).
- Refusals go to the author: on the web as an error in the composer, to
  Mumble clients as PermissionDenied (Text) with the reason, which their
  log shows. There is no moderator channel: blocked words and domains
  are a server log line (`message refused`, without the text).

## Registration captcha (2026-10-18)

- Stateless challenges: the token is salt and end, signed with an HMAC
  key made at start (a restart only voids open challenges). Only used
  ones are kept (at most 100 000, until they end), keyed by the decoded
  payload: Go's base64 decoder skips line breaks, so one payload has
  many spellings.
- The server checks the hash against the level now, not the level in
  the token: raising the level voids easier solutions found before.
- The check comes after the name and password checks (a taken name
  does not use up 1 to 2 s of work) and before bcrypt.
- Calibration: crypto.subtle in a worker did 340 000 SHA-256 a second
  in headless Chrome on an Apple M-series laptop (Node: 185 000).
  One `digest` at a time was half as fast in Node: the solver awaits
  batches of 512.
  Normal is 18 bits (0.8 s there, about 1.5 to 2 s on a mid laptop),
  hard 20 bits. Proof of work varies: 3 times the mean in 5 % of cases.
- crypto.subtle exists only in secure contexts: on plain http (not
  localhost) the check cannot run. Guests then join as classic guests,
  account creation shows the error.
- Classic web guests (no guest account) and Mumble clients do no check;
  they have the connection limits. The setting needs the built-in
  server (as guests read only).
- The worker is a separate file (`new Worker(new URL(...))`), so the
  desktop CSP needs no change: no `worker-src`, so `script-src 'self'`
  applies.
