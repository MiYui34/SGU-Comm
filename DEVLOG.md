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

## Streaming views (2026-10-19)

- The server already lets a viewer watch several streams (viewers are a
  map per stream, one connection carries all). No server change.
- Grid and focus are one keyed list of tiles placed by CSS grid, not two
  lists: switching views or focus never remounts a canvas. A new canvas
  would stay blank until the next decoded frame, and a still screen
  sends few.
- The hidden strip only hides tiles (`display: none`): their streams
  still decode and draw. Simple; a hidden 2D draw is cheap next to the
  decode itself.
- Picture-in-picture is a `canvas.captureStream()` in a video element
  that is never in the page (Chromium takes that). Firefox has no
  Picture-in-Picture API: no button there. The Document PiP API would
  need the canvas moved out of the page; not used.
- Stream pictures with sound wait for the sound clock in an animation
  frame loop, which stops in a hidden tab, so PiP froze while you were
  in another tab. In a hidden tab pictures now show at once (up to the
  0.15 s sound lead out of sync).
- Headless Chromium enters PiP (the promise resolves) but
  did not enter full screen; full screen was not checked there.

## Emoji resize (2026-10-19)

- Still images are scaled in the browser to fit 96×96 (no server
  change: Go's content sniffing already knows WebP). The image is scaled
  once to a working canvas with a short side of at most 96 × 8 (the most
  zoom), so dragging the crop redraws a small canvas, not the photo.
- The output is PNG or WebP, the smaller one. `toBlob("image/webp", 1)`
  is lossless WebP in Chromium, and it gives a PNG where the browser has
  no WebP encoder, so the type of the blob is checked.
- Animation detection walks the GIF blocks (a 0x2C byte can be in the
  image data) and also finds APNG (`acTL` before `IDAT`): a canvas would
  drop those frames too.
- Small images (≤ 96 px, no crop) are uploaded as they are; re-encoding
  them gains nothing.
- agent-browser's `mouse wheel` sends no DOM wheel events (checked with
  a test page), so wheel zoom was not checked there. Drag, keys and the
  slider were.

## Web Push (2026-10-19)

- New dependency: `github.com/SherClockHolmes/webpush-go` (RFC 8291
  encryption and RFC 8292 VAPID; by hand it is easy to get wrong). It
  pulls in `golang-jwt/jwt/v5`. Its `Subscriber` adds `mailto:` to
  anything that is not `https:`: `PUSH_SUBJECT=mailto:x` must lose the
  prefix first.
- Who gets a push is decided in the sidecar (`push.go`): `pings` (name
  mention, reply without no_ping, @everyone, direct message; not the
  author) and `wantsPush` (may read the chat, not connected, not in do
  not disturb). @here never pushes: it is for who is there, and pushes
  only go to who is not.
- "Not connected" is `Voice.Online()` (web and Mumble clients): an open
  tab notifies by itself, so there are no double notifications. A phone
  whose PWA is in the background but still connected gets neither.
- Offline users have no session, so access needs new questions to the
  voice server: `UserSees` and `UserMayRead` build a registered user by
  id (in no channel). Groups and roles by id count; a certificate hash
  or tokens in ACLs do not.
- A subscription belongs to the login session (foreign key, cascade):
  logout, "log out everywhere", password change and expiry end it
  without extra code. The client subscribes again for the new session
  on the next start (`syncPush`).
- The server posts only to the browsers' push services (allowlist of
  hosts); otherwise any account could make it post to internal
  addresses.
- Channel settings and blocks stay in the browser. Only the channels set
  to "Nothing" and the blocked names go to the server, per subscription
  (per device): mutes let mentions through anyway, and pushes are only
  for pings. Without the blocks a blocked user could still ping a closed
  phone.
- Most messages ping nobody: without "@", a reply, @everyone or a direct
  chat, the decision makes no queries.
- Rate: one push per user and chat per minute (in memory), plus the
  `Topic` header, so a push service keeps only the newest per chat.
- The service worker is a second Vite entry (`src/sw.ts` to `/sw.js`),
  registered only while push is on, with no fetch handler: no caching,
  the update check works as before. It must stay one file (no shared
  chunks): only it imports `lib/pushNotice.ts`.
- The WebWorker TypeScript lib clashes with DOM: sw.ts declares the few
  worker types it uses.

## Channel recording (2026-10-19)

- No re-encoding: the voice path hands each packet (copied, the buffers are
  reused) to the recording's goroutine through a channel of 1024; when it
  is full, packets are dropped and counted (`dropped` in recording.json).
  Voice never waits for the disk.
- Alignment: every track starts at the recording's start and ends at its
  stop. Within a burst, Mumble's sequence numbers (10 ms frames) place the
  packets, so network jitter does not move them and lost packets become
  silence. A new burst (after a terminator, a pause over 1 s, or sequence
  numbers more than 2 s off the clock) starts where the arrival time says.
  A burst that starts while the last packet still plays goes after it.
- Silence is explicit packets, not granule gaps: CELT frames `TOC 0xff
  0xfe`. The CELT silence flag is the first symbol in the range coder, so
  the same two bytes are silence at 20, 10, 5 and 2.5 ms; gaps are filled
  with 20 ms frames, then shorter ones (all Opus durations are multiples of
  2.5 ms). Mixing these CELT frames with SILK speech is valid Opus.
- Stereo (music mode) is kept: a track starts mono and its OpusHead is
  rewritten in place (page 0, CRC again, same size) at the first stereo
  packet. Mono packets in a stereo stream decode fine; the other way round
  would be downmixed.
- Pre-skip is 312 (libopus's encoder delay at 48 kHz) on every track, so
  they stay aligned to each other.
- Checked with ffmpeg: real libopus packets with gaps gave two 8.000 s
  tracks (mono and stereo) that decode without errors, with the sound
  exactly where it was spoken (silencedetect).
- Only what a user says to its own channel (target 0) is recorded, if it
  may speak there (not muted, suppressed, or without Speak). Whispers,
  shouts, linked channels and screen share sound are not.
- Downloads: the sidecar gives staff a one-time token (1 min, as stream
  tokens); `/api/recordings/{id}?token=` streams a stored zip (Opus does
  not shrink) and checks again that the session may record. The token is
  in the URL because a download is a plain link.
- A crash leaves a recording without recording.json: it is not listed, and
  removed after the keep period plus the maximum length.

## Flaky mumbled tests (2026-10-19)

- The timeouts were all `superUser` dials ("no frame of type 5", the
  ServerSync), not the registration: helpers call `t.Helper()`, so the
  failure points at the test line. The ServerSync wait (3 s) covers the
  argon2id check of SuperUser's password: 0.3 s under -race, seconds on
  a busy machine. Reproduced with 24 parallel test binaries
  (GOMAXPROCS=2): 46 failures in 960 runs.
- TestWebPasswordChecksShareTheLockout had the same cause: its ten
  failed checks took more than 12 s, so the lockout limiter refilled.
- Fix: tests store a cheap argon2id hash (`setAdminPassword`); Check
  takes the parameters from the hash. 0 failures in the same stress.

## Camera streams (2026-10-19)

- A user has one screen share and one camera at most. The hub keys
  streams by an id: the session for a screen share, the session with
  bit 31 set for a camera (`stream.ID`). Frames up carry a camera flag
  (bit 2); frames down carry the id, so viewers and the channel list
  need no other change. Cameras have no sound. There is no server-wide
  limit on the number of streams; one upload limit (1 MiB/s) covers
  both streams of a user. The server sends it in "ready".
- Watching a camera is opt-in like a screen share (a camera button next
  to LIVE). Simple; Discord shows cameras at once in its voice grid.
- Quality while live: 480p/720p/1080p and 15/30/60 fps, default 720p at
  30 fps (about 4 Mbit/s; screen shares were 1080p at 15 fps, so small
  text is less sharp by default now). The frame loop compares the wanted
  encoder settings with the current ones on each frame. A new size: a
  new encoder (it starts with a keyframe) and a new "stream" config to
  the viewers, who keep their decoder (it takes the new size at the
  keyframe; a new decoder missed that keyframe while it was set up, and
  the server asks for keyframes at most once a second). A new bitrate or
  frame rate only: the same encoder is configured again. Checked in
  headless Chromium: 720p to 1080p60 (screen) and 720p to 480p and back
  (camera) while watched, without an end.
- Alone, a stream may use 80% of the upload limit; screen and camera
  together share it 65/35.
- Camera switch (a phone's front and back): the current camera closes
  before the next opens, because phones open one camera at a time.
- Headless Chromium crashed its renderer on `enumerateDevices`, also
  with the fake media flags. The browser check used an init script that
  replaces getUserMedia, getDisplayMedia and enumerateDevices with
  canvas streams.
- Watching no longer needs getDisplayMedia (it needs decoders only), so
  phones without screen capture can watch too.

## Streaming from OBS (2026-10-19)

- WHIP with pion (new dependency). The stream is an "ingest" in the
  stream hub: viewers see a screen share of the user's voice session; the
  user's own stream connection neither feeds nor ends it. It starts with
  the first keyframe, whose SPS gives the size and the codec name.
- Viewers get the same Annex B chunks the browser encoder sends (no
  decoder description); the server puts the last SPS and PPS in front of
  keyframes that come without them. B-frames are not reordered: OBS should
  send none.
- Each offer gets its own pion API with exactly the offered H.264 and Opus
  parameters, so any H.264 profile matches (pion compares profile ids).
- One UDP port for all sessions (pion's UDP mux), IPv4 only: FFmpeg's WHIP
  client takes only the first candidate, and it was an IPv6 one. FFmpeg
  also read the end of a chunked answer as an error (now Content-Length).
- Picture and sound timestamps come from RTP, each started at its first
  packet's arrival (no RTCP sender reports): in step up to the difference
  in network delay.
- Checked with FFmpeg 9's WHIP muxer (as a stand-in for OBS): the
  forwarded H.264 decodes with FFmpeg without errors. The headless
  Chromium here claims H.264 support and fails on every frame, so the
  picture in a browser was not checked; it now shows the H.264 message.
- Opening Settings in headless Chromium leaves the page (about:blank),
  also before this change: the Settings section was not checked there.

## Playback buffer (2026-10-19)

- Setting "Playback buffer": Low 60 ms (default), Medium 120 ms, High
  250 ms. The maximum queue is the start delay + 240 ms (60 ms gave the
  old 300 ms), so a larger buffer is not cut away.
- Music gets 500 ms (or the setting, if larger): who is in a listen-only
  channel (only speakers can speak there; `listenOnlySessions`), and who
  sends music. Music: per 1 s of audio (durations from the Opus TOC), a
  stereo packet or more than 96 kbps (our speech is 40 kbps mono, Mumble's high preset 72, music
  mode 128 kbps). 3 music seconds in a row switch to music, 10 speech
  seconds back; silence counts for neither, so a jukebox stays music
  between tracks. Counters only, in place: it runs for each packet.
- Each speaker's timing is chosen when its stream starts or stalls, so a
  change never cuts audio mid-stream.
- The connection info offers the next level ("Use Medium (120 ms)")
  from 2 % late audio (the "good" limit). A button that sets the level
  was simpler than opening the settings at the right place. The old late
  audio stays in the 30 s window, so after a change it shows the new
  level instead of a second offer.
- The desktop app's bridge forwards voice to the same web playback, so
  all of this applies there too.

## Offline avatars without 404s (2026-10-19)

- mumble_users.picture caches "the texture is an image Avatars serves",
  so ready can flag members without reading textures. Old databases get
  it filled in the same transaction as the new column (a migration may
  now have a fill function).
- A member "picture" event goes out on every texture save by a
  registered user, also when the flag stays the same: the web only keeps
  the set of ids, and the URL is revalidated by ETag anyway.

## Linux desktop audio (2026-10-19)

- Report: the AppImage had no voice in or out. Reproduced in podman
  (Ubuntu 22.04 arm64, the CI's WebKitGTK 2.50.4) with the real app
  binary and AppImage, Xvfb and PulseAudio null devices. Three causes:
  - No GStreamer plugins in the AppImage. WebKitGTK plays WebAudio,
    records the microphone and does WebCodecs Opus with them. Without:
    "autoaudiosink not found" (the AudioContext stays suspended), no
    capture devices, and isConfigSupported says no to Opus. Fix:
    `bundleMediaFramework` (+10 MB on arm64; the AppImage carries what
    the runner has installed, so CI installs plugins-base and -good;
    pulsesink/pulsesrc are in -good and also serve PipeWire).
  - getUserMedia: NotAllowedError until the app answers WebKitGTK's
    permission-request. media.rs allows UserMediaPermissionRequest for
    our own origins only. enable-media-stream is on by default in 2.50,
    set anyway for older ones. No DeviceInfoPermissionRequest came for
    enumerateDevices (labels come after getUserMedia, as elsewhere).
  - WebCodecs may lack Opus (no plugins, older WebKitGTK): codec.ts
    falls back to libopus-wasm (MIT, 440 kB chunk, loaded only then;
    the CSP already allowed wasm-unsafe-eval). The choice is made once
    per page from isConfigSupported. Encoder and decoder have one small
    interface (planar floats in, packets out); calls made while the
    codec loads wait in order.
- WebKitGTK 2.50 has AudioWorklet, AudioContext at 48 kHz and WebCodecs
  audio; it lists no audio outputs and has no setSinkId, so the output
  picker stays hidden there.
- KALLISTI_DEVTOOLS=1 turns on the web inspector on Linux.

## Native notifications (2026-10-19)

- Report: "desktop notifications not supported" in the Linux AppImage.
  WebKitGTK asks the app for a NotificationPermissionRequest (we only
  answered UserMedia: refused), WKWebView (macOS) has no web Notification
  API, WebView2 (Windows) has one.
- The desktop app now uses tauri-plugin-notification 2.7 (system
  notifications on all three). notify.ts picks a notifier: the plugin in
  the app, the Notification API in browsers, none without it. The
  plugin's permission is read once at start (on desktop it always says
  granted; a block in the system settings is not visible to us).
- The plugin also replaces window.Notification with a shim that calls
  it, so web code that uses the constructor still works in the app, but
  without onclick.
- Clicks: 2.7 reports "tap" on desktop too (macOS and Windows: a thread
  per notification waits for it; Windows only while the toast is on
  screen, not from the Action Center; Linux: D-Bus ActionInvoked). Rust
  brings the window forward (unminimize, show, set_focus); the page maps
  the notification id to its handler and opens the chat. Ids start at a
  random number, so old notifications after a reload do not open the
  wrong chat.
- media.rs also allows NotificationPermissionRequest for our own page
  (with the shim it should not come).
- In the app, a visible window behind other windows counts as not seen:
  notifications for the open chat need the window focus (chatSeen).
- Verified on macOS (release .app, no signing): permission granted and
  the notification sent through the plugin without error. Seeing the
  banner and clicking it was not checked here; Linux (cargo check in
  podman) and Windows by users.

## Paste in the desktop app (2026-10-19)

- Report: pasting an image to attach it works in browsers, not in the
  Linux AppImage. Reproduced in podman (Ubuntu 22.04 arm64, WebKitGTK
  2.50.4, Xvfb; `xclip -t image/png`, `xdotool key ctrl+v`), in a plain
  WebKitGTK view and in the real app binary (tauri://localhost) with a
  probe page.
- WebKitGTK gives the paste event nothing for an image: types [], files
  0, items []. A text paste gives text/plain as usual. In the same paste
  handler `navigator.clipboard.read()` gives the image (image/png, no
  prompt, no permission request, also with wry's default settings).
  With only image/jpeg on the clipboard it gives no types.
- Fix in the web client, no Rust: in the desktop app, a paste without
  files and without text/plain reads the clipboard API and attaches the
  first image as "pasted.png". Checked in the app binary: a 7x5 PNG
  arrives as a File that decodes; a text paste does not read.
- macOS and Windows give pasted images as files, so the read runs there
  only for a paste with neither files nor text (an empty clipboard, for
  example). WebView2 may then ask for clipboard access, WKWebView may show its
  "Paste" callout (not tested).

## Music buffer from the first packet (2026-10-19)

- A user heard ~5 s of rough audio in the jukebox channel after joining:
  music detection needed 3 s, and a new buffer applies only at the next
  stall. Stereo (the jukebox sends 128 kbps stereo; speech is mono) now
  counts as music from the first packet, so the stream starts with the
  music buffer. The bitrate rule keeps its 3 s.
- The "larger playback buffer" hint counts only late audio of speech: the
  music buffer covers music speakers, the setting does not change them.

## Audio after start (2026-10-19)

- Setup: local built-in server, 25 registered Mumble users with avatars
  (random-noise PNGs, 50-120 KB, 2 MB in all) in "General", one of them
  talking without pause (20 ms Opus frames), 400 messages in "general"
  (every 4th with a 70 KB JPEG). Listener: headless Chromium over raw
  CDP, a guest who joins General at once; a hook counted per Mumble
  message type and JSON event the bytes and arrival times on the socket,
  and sampled playback's good/late/lost every 100 ms. Slow link: a TCP
  proxy that gives all connections one bottleneck (8 or 3 Mbit/s down,
  20 ms each way); Chromium's network throttling was not used. CPU x4:
  CDP CPU throttling. 15 s per run, fresh profile (no cache).
- Before, 8 Mbit/s: in the first 2.3 s the socket carried 2.0 MB of
  avatar blobs (RequestBlob answers for all online users), then voice;
  JSON (ready, one history page, voice chat) was 12 KB. Voice reached
  the listener 2.2 s after the join, as a burst: 96 late (dropped)
  chunks, then smooth. 3 Mbit/s: 5.6 s and 270 late chunks, the
  reported "jerky for about 5 s, smooth once the history is in" (the
  history page itself comes early: it goes out beside the blobs).
  Unthrottled with CPU x4: 0.6 s and 19 late. So the cause was head-of-
  line blocking in the per-client queue, not main-thread work.
- After (a, below), same runs: voice 0.08 s (8 Mbit/s) and 0.15 s
  (3 Mbit/s) after the join, 0 late chunks in 10 s (one run: 1), also
  with CPU x4 and x6. The avatars still cross the same link over HTTP,
  but no longer queue ahead of voice. Long tasks stayed at 140-170 ms
  (CPU x1) and 370-640 ms (x4-x6) in the first 10 s without late audio,
  so main-thread work (d) and deferring history or previews (b) were not
  done: the numbers do not show them.
- (a) Online registered users' pictures come from /api/avatars/<name>
  ?v=<texture hash> (from UserState) instead of RequestBlob; the hash
  makes them cacheable for good. For online users the endpoint serves
  the live texture: the database gets it a moment after the UserState
  with the new hash, and a client that asked in between would keep the
  old picture (or a cached 404). ready's `avatars` says the server
  serves them (built-in server without a password: as for offline
  members, a private server keeps them private). The client asks for
  textures only after ready, or 3 s after the sync without one (no
  Kallisti server); guests and servers without the flag still use
  RequestBlob.
- (c) mumbled sends queued voice before other queued frames. Without
  HTTP avatars (private server): voice starts after 0.7 s instead of
  2.2 s (8 Mbit/s), late chunks 97 -> 47; at 3 Mbit/s 271 -> 225 with
  gaps up to 1 s. Most of the backlog is below the queue (the pipe to the
  bridge, the bridge's 32 KB writes, the TCP send buffer), so this only
  helps a little; Murmur upstreams get nothing from it. Voice can now
  come before ServerSync, so the web client plays voice only once
  synced (local mutes and volumes are set then).
- Not measured: WebKitGTK (Chromium only), the desktop app's UDP voice.

## Linux notifications: icon and button (2026-10-19)

- Report (AppImage): a broken icon and an empty button in the
  notification. Reproduced in podman (Ubuntu 22.04 arm64, Xvfb) with the
  plugin's D-Bus call: app_name and app_icon "kallisti-desktop" (the exe
  name; no such theme icon in an AppImage), actions ["default", ""].
  lxqt-notificationd shows an empty button for it, xfce4-notifyd a
  missing-image icon; dunst and notification-daemon show no icon.
- tauri-plugin-notification 2.7 adds the "default" action with an empty
  label itself, so on Linux the app now sends notifications over D-Bus
  (notify.rs, zbus like the plugin): app_name "Kallisti", app_icon a
  file:// URI of icons/128x128.png written to the cache dir at start,
  actions ["default", "Open"]. Same connection for sending and signals
  (dunst), kept open (GNOME). macOS and Windows still use the plugin.
- The page calls the app's `notify` command on all desktops; a click
  comes as the "notification-action" event (the plugin's onAction
  shape), after Rust brought the window forward.
- Checked with the release binary and a probe page using nativeNotifier
  in lxqt-notificationd, xfce4-notifyd, notification-daemon and dunst:
  icon shown, an "Open" button in LXQt and xfce (notification-daemon
  and dunst show none), a click restores the minimized window and runs
  the page's handler. LXQt keeps the notification open after the click.

## DM context menu (2026-10-19)

- Right-click, long press or the context-menu key on a direct chat in
  the sidebar: Profile, Mark as read (only when unread), Mute (the
  channel durations), Pin to top / Unpin, Close conversation. No
  delete: the other person keeps the chat. ChannelMenu got a `direct`
  mode instead of a second menu component (same styles, same mute block).
- Closed and pinned chats are kept on the device (localStorage
  `directChats`, per server), like channel mutes. Closed stores the
  newest message id at close time: a newer message lists the chat
  again, and so does opening it (user menu "Message", a link). Pure
  logic in lib/directList.ts. Server-side (per account, synced) would
  need a new table: read_marks_by_user only holds a message id per chat,
  so it cannot store "closed" or a pin order. A later option.
- Mutes reuse channelPrefs with the DM's chat id. A muted DM has no
  desktop notification, no sound, no badge or bold and no tab title
  count. Only mutes "until I turn it back on" go to the push "quiet"
  list: the client sends the list only while a tab is open, so a timed
  mute would stay quiet on the server after it ended. Notification levels are not offered:
  every DM message is for us, so only mute makes sense.
- Closing marks the chat read: else its unread count stays in the tab
  title with no row to show it.
- Gap: Ctrl+K does not find a closed chat whose partner is offline (an
  online one shows as a user).

## Desktop app hangs (2026-10-19)

- Report: the Linux AppImage hung, for good or until "WebProcess didn't
  exit as expected after the UI process connection was closed", when the
  user joined the Music channel and when the user Settings opened.
- Cause: WebKitGTK starts and stops its GStreamer audio output
  (AudioDestinationGStreamer) with webkitGstSetElementStateSynchronously,
  which waits on the pipeline's bus on the page's main thread without a
  time limit. When a context gets its first AudioWorklet module, WebKit
  restarts a running output (BaseAudioContext::workletIsReady ->
  restartRendering: a stop, then a start). Our context starts at login,
  and the capture worklet came when the microphone opened first: at a
  voice join, or when the Settings open (level meter). Both reports came
  at that moment.
- Repro: podman, Ubuntu 24.04 with PipeWire 1.0.5 and WirePlumber, Xvfb,
  the real arm64 AppImage (its WebKitGTK 2.50.4 and GStreamer 1.20), the
  built-in server and a stereo 128 kbps sender in the container. A
  script acts as a Bluetooth headset that changes profile when a
  microphone opens: the output sink goes away and a new one comes 20 ms
  later. Settings: 4 hangs in 10 runs; voice join with the music
  stream: 2 in 8. Stacks: the main thread in gst_bus_timed_pop_filtered
  (from the restart), the AudioWorklet thread (the new render thread) in
  appsrc's push, the sink waiting on its clock. Closing the window then
  prints the reported message. A PipeWire without a session manager
  hangs the same wait already at the first start.
- Fix: in the Linux app the context is suspended at once, gets the
  capture module, then resumes. suspend() marks it "suspended by script"
  before WebKit's lazy start, so the output starts once, after the
  module, and the restart has nothing to do. Same repro: 0 hangs in 10
  and 8 runs (and 6 with the decoder change below); the GStreamer log has one start at login and none at the
  voice join or in the Settings. Browsers keep the old order: Safari on
  iOS keeps a resume() outside a tap suspended.
- Left: the start at login and the stop at logout still wait without a
  limit; a sound server that does not answer then still hangs the app
  (a WebKitGTK bug).
- Memory: WebCodecs decoding in WebKitGTK (2.50.4 and 2.52.6) keeps
  about 16 KB per stereo 20 ms frame in AudioData.copyTo to f32-planar;
  a copy in the data's own format (f32) crashes the web process ("free():
  invalid pointer"). Decoding without copyTo and the encoder stay flat.
  With the music stream: +0.8 MB/s, 3 GB an hour. The Linux app now
  decodes with libopus (wasm): flat at about 230 MB over 90 s of music,
  with sound on the output. Encoding stays on WebCodecs.
- Headless Chromium dying on Settings: agent-browser starts Chrome with
  --single-process. There, enumerateDevices or a devicechange listener
  kill the whole browser on any page (SIGTRAP; getUserMedia: SIGSEGV),
  and Settings use both when they open. A normal headless Chrome runs the
  same calls, so the cause is not in our code. Tests in agent-browser
  must stub all three (getUserMedia, enumerateDevices and
  mediaDevices.addEventListener); then Settings open.

## Long message collapse (2026-10-19)

- A message collapses when its written text has more than 20 lines; a
  line longer than 120 characters counts as more lines (lib/collapse).
  Code blocks count with their lines. A full 2000-character line is 17
  lines: it stays open on a wide screen (on a phone it is taller).
- Mumble HTML has no source text: its lines come from htmlToLines, and
  its measured height also counts (over 400 px, again after each image
  load): htmlToLines folds blank lines, and images have no lines.
- Collapsed: 12 lines, faded with mask-image, then "Show more". The text
  is only clipped, so screen readers read all of it; focus (Tab) in it
  expands it. Expanding keeps the scroll place; "Show less" scrolls the
  line back into view.
- Search results and pins are not collapsed: they show plain text with
  the newlines folded, so they stay short.

## Join and leave sounds (2026-10-19)

- Sounds for others joining or leaving our voice channel, our own join
  (also a move into another channel) and leave, mute and deafen. Our own
  notes, synthesized like the mention ping (ping.ts now has a shared
  `playNotes`): rising for joins and unmuting, falling for leaves and
  muting, three notes for our own steps, lower ones for deafening.
- Pure logic in lib/sounds.ts: `voiceSound(prev, next)` compares two
  snapshots of the state and gives at most one sound. No previous
  snapshot (the sync at login: Client is mounted per connection) gives
  none. Deafen changes win over everything; while deafened nothing else
  plays. Others only count when they enter or leave our channel.
- Burst limit: one sound per 500 ms (`allowSound`), for all sounds.
  After a reconnect none for about 3 s: the server restart brings
  everyone back at once, and the client goes back into its old channel
  and mute state (which would give a join and a mute sound).
- Setting "Sounds" (on by default) with a volume slider in Settings.
  Played on the chosen output device (setSinkId where it exists).
- Checked in headless Chromium against a local server, with
  createOscillator wrapped to log the notes: join, mute, another user's
  login into the lobby (silent), join and leave, deafen and undeafen.
- Gap: a reconnect was not tested by hand.

## Who reads a private channel (2026-10-19)

- No new event: the "private" list (sent again on every access, member
  and role change) now has the members also for the channel's members.
  Others who see the channel (by an ACL from a Mumble client) still get
  it without members; the client then shows the normal list, as it
  cannot tell "no members" from "not told".
- Staff come from the roles list, which everyone already gets. Each user
  shows once: a staff member who is also a member is under Staff.
- Groups keep online users first, offline ones faded below, instead of
  four headings (Members online/offline, Staff online/offline).

## Word replacements (2026-10-19)

- Part of the message rules (`Rules.Replacements`, the same JSON
  setting and admin tab). `msgrules.Replace` runs before `Check` in
  `checkMessageLocked`, so `CheckMessage` now gives the text to post. The
  sidecar puts it into the request (send, edit, upload caption); the
  server stores it, so the author sees what everyone sees. Mumble texts:
  unescape, replace, escape again; the web hooks get the replaced text.
- For everyone, staff too: it is for fun, not moderation. Blocks and
  limits stay staff-exempt. A replacement can contain a blocked word;
  then the message is refused (the admin's choice).
- One match: one replacement at random (`math/rand/v2`; tests inject
  `pick`). Case: all capitals (two letters or more) give capitals, a
  capital first letter gives one; else the replacement as written.
- Too long after the replacement: refused, not cut (the web composer
  gets its text back; Mumble: TextTooLong). Cutting would lose the end
  without a word.
- No injection: replacements may only have letters, digits, marks,
  emoji (So, skin tones, ZWJ, VS16), spaces and ' . , ! ? - (checked on
  the server and in the tab). Nothing that formats (* _ ~ ` | >), links
  (: /), mentions (@ #) or is HTML. The message format has no escapes,
  so a check is simpler than escaping. Nothing is replaced in code,
  links, @mentions, #channels and :emoji: names, and a phrase matches
  only with separators (spaces . - _) between its words, so "**nice**
  ass" keeps its markup ("**nice** behind" with an entry for "ass").
- Matching (also for blocked words, which gain all of this): each
  character with its combining marks is folded on its own (NFKC, marks
  off Latin, Greek and Cyrillic letters and off what is no letter or
  digit, case folding, lookalikes), with a byte map back to the
  original, so matches replace the original text. Lookalikes: a small
  table of lower-case letters, applied after case folding so both cases
  match (Cyrillic а е о р с у х and в н к м т as small capitals, Greek
  α ο ν ρ and others, Latin small capitals, dotless i); full-width and
  mathematical letters come from NFKC. Greek Μ and Ζ stay (their small
  letters look like nothing Latin).
  Single letters with only spaces, dots, dashes or underscores between
  them are one word ("a s s", "a.s.s"); a single letter next to a longer
  word stays a word ("a spam"). A letter three times or more also
  matches fewer of it ("asss", "spaaam"); twice does not ("too" is not
  "to", "good" is not "god"). Whole words only: "class", "assistant",
  "passage", "bass" stay.
- Left: Russian or Greek words that fold to a Latin entry match ("сор"
  is "cop"); ß folds to ss ("aß" is "ass", as before); joined letters can
  make a word of initials ("A. S. S."); Hangul as separate jamo and
  half-width kana with separate marks do not match; the rules' entries
  are folded again for each message (fast enough for 500).

## Join and leave sounds dropped (2026-10-20)

- Users found the join and leave sounds annoying; they are gone (ours and
  others'). Mute and deafen sounds stay: they confirm one's own action.

## Stream playout buffer (2026-10-20)

- Report: a shared YouTube tab (with sound) is "mostly fine, sometimes
  choppy" for a viewer in America (server in Helsinki, 120-180 ms RTT: a
  lost TCP segment stalls the stream for 200-400 ms).
- Setup: `tests/perf/stream.sh` (see tests/perf/README.md): a streamer
  in headless Chromium shares a canvas with high motion (720p30, the
  default quality: 3.8 Mbit/s VP8), one viewer direct and one behind
  linkproxy, which now has jitter (`-jitter`, `-jitter-chance` for
  spikes). 20 s per run, one run each, a Mac. Scenarios (viewer link):
  far = 10 Mbit/s, 150 ms RTT, spikes of 200-400 ms; jitter = 8 Mbit/s,
  0-200 ms always; tight = 5 Mbit/s, 150 ms RTT, spikes of 150-300 ms;
  spiky = 10 Mbit/s, 150 ms RTT, spikes of 400-800 ms; slow = 3 Mbit/s
  (below the bitrate). local: two viewers without a proxy.
- Before, the viewer behind the proxy (freezes: gaps over 200 ms between
  drawn pictures, count and total; latency: send to draw, median):

  | Run | Freezes | Other | Latency ms |
  | --- | --- | --- | --- |
  | local, sound / silent | 0 / 0 | first picture after 7.8 / 7.5 s | 207 / 4 |
  | far, sound / silent | 0 / 4 (1261 ms) | | 580 / 94 |
  | jitter, sound / silent | 0 / 1 (240 ms) | | 446 / 166 |
  | tight, sound / silent | 0 / 6 (1539 ms) | 2 sound packets late | 396 / 108 |
  | spiky, sound / silent | 5 (1968 ms) / 1 (441 ms) | sound: 41 pictures not drawn, 5 keyframe asks | 961 / 93 |
  | slow, sound / silent | 5 (7058 ms) / 2 (496 ms) | 227 / 30 lost, keyframe wait 8 s / 1 s | 1417 / 1754 |

- Causes, from these numbers: (1) a stream without sound had no buffer
  at all (pictures drawn on decode), so every spike froze it. With sound,
  the 0.15 s lead plus the sound's own delay (pictures were 0.2-0.4 s
  behind their capture) covered spikes up to 400 ms, not 800. (2) After
  an 800 ms stall the backlog came as a burst; more than 10 frames in
  the decoder made the viewer drop to the next keyframe and ask for one
  (5 asks, 41 pictures not drawn). The sound clock's new start after 5
  late packets did not show here (0 jumps), only on the slow link (37).
  (3) Keyframe waits: a new viewer's keyframe often came before its
  decoder was set up, and the decoder's own request fell in the server's
  one-a-second limit and was lost: the viewer waited for the periodic
  keyframe (every 8 s). A viewer whose queue was full asked at once,
  and that keyframe found the queue still full: again up to 8 s. (4) The
  streamer was not the cause: the encoder queue stayed at 0, no frame
  was skipped, keyframes were 83-95 KB (the upload limit is 1 MiB/s with
  2 MiB burst), and the direct viewer lost nothing. Not measured on a
  slow streamer machine.
- (a) Playout (`lib/stream/playout.ts`): picture and sound on one clock,
  the audio context's with sound, else the page's (silent streams get the
  buffer too). The first item, picture or sound, sets it: it plays a lead
  of 0.5 s after it came. A late item raises the lead in steps of 0.25 s
  (up to 1.5 s) and plays at once: a pause where the stall was, not a
  skip; sound and pictures already scheduled keep their times, so both
  pause at the same place. After 60 s without late items the lead goes
  down by 5 ms a second (sound buffers play 0.5 % faster meanwhile, or
  they would overlap), while the slowest item of that time still had
  over 0.5 s to spare. Still late at 1.5 s: dropped;
  late for 1 s: the clock starts over (a delay that stays); an item over
  1 s beyond the lead: starts over (the clock jumped, or a backlog
  drained).
- Pictures wait encoded and go to the decoder 0.1 s before their time:
  decoded VideoFrames are large (1.4 MB at 720p) and hardware decoders
  stall when many are held. Late pictures are still decoded (the next
  ones need them) but not drawn. At most 240 encoded pictures wait; the
  decoder may hold 30 (was 10, which bursts after a stall passed).
- One loop draws all watched streams: animation frames, and in a hidden
  tab the worker timer (no animation frames there), so picture-in-
  picture stays in sync with the sound; before, a hidden tab drew
  pictures at once.
- (b) Server: a keyframe request within the limit is kept and sent with
  the first frame after it (the limit is now 0.5 s, was 1 s). A viewer
  that was full asks for its keyframe when its queue has room again
  (at most a quarter full), not at once, and at most every 2 s: without
  that limit one slow viewer brought keyframes for all twice a second
  (11 requests in 17 s on the slow link; 4 with it). The periodic
  keyframe stays at 8 s: with these two, recovery did not need it (0
  requests from the server in all runs but slow).
- After (same runs; a/v: a picture is drawn 0-12 ms after its sound is
  scheduled, p5 to p95; the first picture now comes at least the lead
  later):

  | Run | Freezes | Other | Latency ms |
  | --- | --- | --- | --- |
  | local, sound / silent | 0 / 0 | first picture after 0.6 / 0.6 s | 526 / 509 |
  | far, sound / silent | 0 / 0 | | 585 / 660 |
  | jitter, sound / silent | 0 / 0 | | 660 / 668 |
  | tight, sound / silent | 0 / 0 | | 584 / 952 |
  | spiky, sound / silent | 2 (567 ms) / 1 (275 ms) | 0 not drawn, 0 asks | 1078 / 910 |
  | slow, sound / silent | 6 (5282 ms) / 5 (3801 ms) | 98 / 41 lost, longest wait 2.7 / 1.5 s | 3371 / 3521 |

  An 800 ms spike still pauses (by what the lead does not cover yet);
  then the lead is higher and later ones pass.
- (c) Adaptive bitrate: not done. Only a link below the stream's bitrate
  (slow) still stutters, and a buffer cannot help there: the server's
  queue fills, drops until a keyframe, and the cycle repeats. With sound
  a little better, silent worse (probably because the keyframe comes
  sooner, so the overload starts again sooner); latency 3.5 s. The fix
  is a lower bitrate: the server would tell the streamer from viewer
  drops, but one encoder serves all viewers, so one slow viewer lowers
  the quality for all (simulcast would avoid that). A follow-up.
- Gaps: Chromium only (no Firefox, Safari, desktop app); hidden tabs and
  picture-in-picture not checked headless; the slow lowering of the lead
  is only unit-tested (runs are 20 s); one run per scenario; latency is
  now 0.5-1.1 s (was near 0 without sound, 0.2-1 s with), as wanted for
  one-way watching; an audio context that stays suspended would hold
  the pictures of a stream with sound (as before). Pictures follow the sound's schedule, not what
  is heard: the output latency (large on Bluetooth) puts them early, as
  before. A new start of the clock that moves it earlier (rare: a
  backlog drained by over 1 s) lets sound already scheduled overlap the
  new for up to a second.

## Review fixes: client (2026-10-20)

- Stream playout: after a late item the lead went down by the drift of all
  the time since, also with no item in between. A still screen share
  (one picture after 5 min) lost its next picture. Now the lead goes down
  only after an on-time item was seen since the late one, and by at most
  1 s of drift per item. Recording the late item's own slack (as the
  review proposed) was not done: it is below one step by design, so the
  lead would never go down again.
- Voice codecs: a failure (load, call, or the codec's error callback) is
  for good. Queued calls (with their samples) are dropped, later ones
  ignored, the error is reported once. Capture drops the failed
  encoder's pending inputs and sends nothing until the next talk spurt,
  which tries a new encoder (not at once: a codec that cannot load would
  fail in a loop).
- Playback drops voice of a session until the client set its local mute
  and volume (setUserGain, from the UserState and the per-name settings).
  Before, voice that overtook a muted user's UserState after a
  reconnect played at full volume until the mute came. The first packets
  of a new speaker (until the next effect run) are lost.
- Avatars: the server serves (and flags as pictures) only images of at
  most 1024 px a side, online and offline. It reads the header only
  (image.DecodeConfig; WebP from golang.org/x/image). Gap: databases that
  flagged a larger picture before keep the flag until the next save; the
  client then gets a 404 (cached 5 min).

## Review fixes: push (2026-10-20)

- Sending: 16 workers and a queue of 1024 pushes (was a goroutine per
  push that waited for one of 16 slots, so a slow push service piled
  them up). The one-per-minute collapse comes before the queue; a full
  queue drops the push (counted, logged on the first and every 100th).
  On stop, sends on their way are cancelled and the queue is dropped.
- Banned users get no pushes: offline access (`UserSees`, `UserMayRead`)
  and the new `UserBanned` check the user's certificate against the ban
  list. Direct messages need no access check, so the hub asks
  `UserBanned` itself. An address ban cannot count for a user who is not
  connected (no address); a registration without a certificate is only
  banned by address, so it still gets pushes.
- Client: subscribing (start sync, prefs, the switch) runs one at a
  time. Turning off does not wait in that line (a stuck
  `serviceWorker.ready` must not keep it from the logout): it counts up,
  and subscribing asked for before it stops, or unsubscribes, sends the
  DELETE and unregisters what it made. Subscribing asked for later waits
  for the turning off. A `ready` that never comes now holds all later
  subscribing in that tab (before: only that one call).

## Review fixes: streams and recording (2026-10-20)

- WHIP: Pion's H.264 depacketizer appends FU-A fragments until an end
  fragment, also across pictures, so unfinished starts grew memory without
  bound. A wrapper drops an unfinished unit at each start and one over
  1 MB (the hub's frame limit); its picture counts as lost (keyframe
  asked for). Pictures over 1 MB are lost too.
- WHIP: the stream key is checked again under the lock EndAccount takes
  when the session is reserved: a key revoked while the offer came in
  (slowly) starts nothing. A track's end ends the session (bad packets
  are skipped; before, they ended the reader), and a live stream without
  pictures for 10 s ends (it froze).
- Recording: at most 50 tracks (each an open file until the end); later
  speakers are not recorded and are named in recording.json
  (`unrecorded`). A write error stops the recording ("write error", told
  like the limits). The size limit counts the page still buffered, and
  the silence that ends the tracks: it goes a second at a time on all
  tracks and stops at the limit, so the recording ends earlier ("size
  limit"). A stop that finishes while a newer recording runs is not told
  ("Recording stopped" would be false).
- Mumble port: an Accept error (out of file descriptors) no longer ends
  the accept loop: it waits (5 ms, doubling to 1 s) and tries again;
  only a closed listener ends it.

## Avatar flags and image size limits (2026-10-20)

- The picture flags of registered users are checked again once at start
  (`pictures_checked` in mumble_meta, one transaction): avatars flagged
  before the 1024 px check, or broken ones, lose the flag (no more 404s).
- imagesize: one header-only check (image.DecodeConfig) for all images.
  Inline images (chat, link previews, custom emoji) have at most
  16384 px a side and 2^26 pixels (256 MiB decoded): panoramas and
  "50 MP" phone photos (8192x6144) pass, pixel bombs do not. Avatars
  stay at 1024 px. Rare JPEGs Go cannot read (arithmetic coding,
  12 bit) count as broken.
- Mumble texts: an oversized picture is dropped, its place gets
  "[image too large]"; the text stays. Pictures whose header does not
  decode are dropped without a note (they are no real images).
- Uploads: an oversized image is kept, but stored as
  application/octet-stream, so it is a download and the web app shows a
  file card. Friendlier than a refusal: the file still gets through.
  The check reads the stored file, as a JPEG's size can come after
  the first 512 bytes. Uploads and emoji from before are not checked.
- Link previews: an oversized or broken image leaves the card without
  one. Custom emoji: refused (413, broken: 415).

## Channel moderators (2026-10-20)

- One source of truth: a "channelmod" group in the Mumble channel (a voice
  channel, or a text channel's mirror) and one ACL entry for it, here and
  in subchannels: Traverse, Enter (they see a private channel), MuteDeafen
  and Move. No Write: with it they could edit the ACL and appoint
  moderators. Kallisti's rights ask the same group (ModeratesIn); Mumble
  groups inherit, so subchannels count. The entry goes with the last
  moderator.
- Rights: delete and pin messages, a text channel's topic and slow mode
  (they are exempt from it there), server mute, move out, the members and
  invite links of a private channel (not making it private or public),
  and channel time-outs. Voice channel chats have no topic or slow mode.
- The voice "kick" is a disconnect: a move to root (the lobby). A server
  kick or ban is server-wide, so it stays with staff. Mass delete stays
  with staff too: it spans all channels.
- Never over staff, and not over the same channel's other moderators:
  native Mumble mutes and moves check this too (State.shields), as Mumble's
  ACLs alone would allow it. Since then, nobody without staff rights
  (also a temporary channel's maker, who has Write there) mutes or moves
  staff. Staff act on channel moderators; nobody acts on SuperUser.
- A moderator must see the channel: a private subchannel stays closed to
  the parent's moderators (Mumble groups inherit, but the subchannel
  denies Traverse). Only staff change a "channelmod" group in a Mumble ACL
  edit (others keep the old one): a temporary channel's maker has Write
  there and could appoint himself. A user timed out in a channel cannot
  lift the suppress there.
- Deletes and pins look at the chat, not at the author: a channel's
  moderators also delete staff messages there (as Discord's Manage
  Messages). Their mutes are server mutes (Murmur's), so they stay when
  the user leaves the channel.
- Appointed by admins and moderators in the channel menu (Moderators…),
  with the members dialog (registered users). Changes go out with the
  roles event (channelModerators: each channel's own moderators, only of
  channels the user sees); every ACL edit now sends it, as groups of any
  channel can hold moderators. The member list tags them (an outlined
  "Mod") only in their channels.
- Channel time-outs share the time-out map and timers (a key with the
  channel) and are saved in mumble_channel_timeouts (no foreign key:
  temporary channels are not saved; time-outs of channels that are gone
  are skipped at the start). They take Speak, Whisper and TextMessage in
  the channel and below (State.Permissions), so Mumble clients see the
  rights, users there are suppressed, and the sidecar's MayWrite and
  MaySpeak follow. Text channels now also need TextMessage in their mirror
  to write. They come in the timeouts event as channelTimeouts (older
  clients ignore them). A new request type (channel-time-out), so an
  older server refuses it instead of timing out server-wide.
- Channel moderators of private channels pick new members from the
  members list (registered users but guest accounts): Mumble's user list
  needs Register in root.
- Known gaps: the web walks the channel tree for inherited moderators and
  ignores a group's Remove entries from Mumble ACL edits. A Mumble client
  timed out in a channel cannot send private texts to users in that
  channel either (Murmur checks TextMessage in the recipient's channel).

## Voice subchannels of channel moderators (2026-10-20)

- Channel moderators make permanent voice channels in the voice channels
  they moderate, at any depth. Below the moderated channel they rename,
  reorder, move and delete them (also temporary ones there). The
  moderated channel itself stays as it is, and nothing leaves or enters
  the subtree. Their moderator rights cover the new channels, as groups
  inherit.
- Not ACL entries: State.Permissions adds the rights (modPermissions):
  MakeChannel where they moderate, Write where they also moderate the
  parent. Mumble clients see them in PermissionQuery and use their own
  menus; the web follows the same bits. Write from the ACLs would let
  them edit the ACL, so ACL edits (mayEditACL) and keepWrite ask the ACLs
  only, as do listen-only and Mumble's user lookup (QueryUsers); a
  channel made with a moderator's MakeChannel gives its maker no admin
  group or Write entry. Sees asks the ACLs' Traverse (Permissions asks
  Sees for moderators). Mumble clients show the ACL tab in subchannels;
  the server refuses it.
- Moves need Write and the new parent's MakeChannel both from the ACLs or
  both a moderator's (mayMove): no channel of their temporary channel
  goes in, none of theirs out. A move or delete with a moderator's Write
  needs it in every channel below too (modWritesAll): a private
  subchannel they do not see stays where it is.
- Private subchannels: yes. create-private-voice takes the parent
  (voice); a moderator who makes one is its first member, else it would
  not see it (private channels are closed to a parent's moderators).
  Making a channel private or public stays with staff.
- Text channel mirrors and everything below their parent get none of
  this (State.Mirror).
- The rights are added to the ACLs' (not filtered by them): an ACL deny
  for "channelmod" does not take them away.
- Limits (State.modLimit, like Murmur's channelcountlimit and
  channelnestinglimit): a channel made with a moderator's MakeChannel is
  refused if the moderated channel has 50 channels below it, or if it
  would be more than 5 levels below it. The moderated channel is the
  topmost of the moderator's above the parent, the one it may not edit;
  with nested moderated channels the outer one counts. Mumble clients
  get ChannelCountLimit / NestingLimit with the reason, the web the same
  text as a manage error. Staff are not limited. A moderator's move
  within the subtree is refused (NestingLimit) if the moved channel's
  deepest one would be more than 5 levels below; the count does not
  change, as nothing enters the subtree. The web has no channel moves.
- Known gaps: two mirror checks (isMirrorLocked: the parent and its children,
  State.Mirror: any depth).

## Text categories (2026-10-20)

- One level, like Discord. A store table (categories: name, a unique
  name key as Mumble compares names, position, Mumble channel) and a
  category column on text channels. Old databases: every channel is
  uncategorized; the mirrors stay where they are until moved.
- Mumble side: a channel per category under "Text channels", after the
  uncategorized mirrors, with its channels' mirrors as children. One
  sync call (SyncTextTree) places everything: a mirror moves when its
  category changes. A category denies TextMessage in itself only; the
  parent's deny of Speak, Write and MakeChannel reaches it and its
  mirrors. Mumble clients can enter a category channel but not write
  there.
- Category moderators are the same "channelmod" group, on the category's
  channel. Mumble groups inherit, so they moderate all its channels and
  a channel time-out given there holds in all of them, with no new code.
  A private channel in a category stays closed to the category's
  moderators unless they are members (as with private subchannels).
- Rights: staff create, rename, order and delete categories, and move
  channels between them (the edit dialog's category field). A category's
  moderators create (not private) channels in it, and rename, order and
  delete the channels in it that they see. They may not bring back a
  deleted channel's history: CreateChannel revives a hidden channel only
  for staff (it may have been private).
- Deleting a category moves its channels to the top, without a category
  (not refused). Mirrors move to the parent before the category channel
  goes, and clients learn the move first: otherwise a Mumble client drops
  the mirror with the users in it.
- One mirror check (inTextTree, State.Mirror): the mirrors' parent and
  everything below it, at any depth. Categories and their mirrors get no
  voice rules (listen-only, recording) and no channel moderator
  subchannel rights (MakeChannel, Write).
- Ordering a category's channels puts them into the places they had in
  the global order; the others keep theirs.
- Clients get the categories with ready and every channels event, only
  those with a channel the user reads and those it moderates (staff:
  all). Collapsed categories are kept per server on the device
  (localStorage "collapsedCategories"); a collapsed category still shows
  the open channel and unread ones. Alt+Up/Down and the quick switcher
  follow the sidebar's order; the switcher shows the category's name.
- Manual check (headless browser): created a category, a channel in it,
  moved a channel into it, collapsed it (kept after a reload), appointed
  a moderator who then created, moved and edited channels only there,
  and deleted the category (its channels moved to the top).
- Known gaps: no drag and drop (menu and dialog only). No private
  categories in the app (the server does not refuse set-private on a
  category's channel, but that is untested). A category's moderators
  can learn that a channel name is taken (also a private or deleted channel's) from the create error.

## Threads (2026-10-20)

- A thread is a chat of its own: a hidden channels row (its id is the
  thread's id) plus a threads row (text channel, start message, name,
  maker, archived). History, send, uploads, reactions, pins, typing,
  read marks and search go through the usual chat paths. The sidecar's
  chat has a parent; may(), mayModerate (the parent's mirror) and the
  push check use the parent text channel, so private channels, channel
  and category moderators, read-only guests, time-outs, slow mode and
  message rules hold as in the channel. One level: only messages of text
  channels start threads (no threads in voice chats, direct chats or
  threads).
- Old databases get the new tables at the start. Store queries that mean
  "text channel" use one predicate (textOnly), so channel changes never
  touch a thread's row.
- Rights: who may write in the channel starts a thread (it takes a slow
  mode turn there, and the message rules check the name). Its maker
  renames and archives it; staff and the channel's moderators also delete
  it. Slow mode: the channel's time, with turns per thread.
- Deleting the start message keeps the thread ("The original message was
  deleted" in its pane). Deleting (hiding) the channel hides its threads
  too; they come back if staff bring the channel back. Deleting a thread
  removes its messages and files, but keeps the hidden row (renamed):
  SQLite would give a deleted row's id again, and clients keep ids.
- Following: the maker, those who write in a thread and those it
  mentions or answers (not @everyone) follow it; the follow event goes
  out before the message. Followed threads count as unread, and their
  summary comes with ready; others show in the list without unread
  marks. Muting and notification levels are the client's per chat id, as
  for channels. Push notifications: mentions, as in channels.
- Thread settings follow the channel: a thread without its own mute and
  level takes its channel's (withParents, used for unread marks,
  notifications, the title count and the mention sound). Its menu sets an
  own setting, stored whole with its level (so "Only @mentions" or no
  mute also hold against the channel); "Same as the channel" forgets it.
  Push: the server resolves it, as the client does not know every thread
  (new or archived ones while no tab is open). The note has the thread's
  channel; a device's quiet channel keeps its threads quiet unless the
  device lists the thread as loud (a chat with a level of its own).
- Archive: a minute timer (main.go) archives threads without a message
  for the staff setting (1 h, 1 d, 3 d (default), 1 w; store_meta). A new
  message opens a thread again. Archived threads stay readable and
  searchable, in the channel's thread list (paged by id).
- Wire: ready and channels events carry the active threads of channels
  the user reads; ready has follows and threadArchive. Events: thread
  (made or changed: name, replies, last reply, archived, start quote),
  thread-deleted, follows, archived-threads, thread-archive. Messages
  carry the thread they started; search results carry the threads of
  their messages.
- Web: "Thread" in a message's actions asks for a name (the start of the
  message). The thread opens in the right column on desktop (one ChatPane
  in compact mode), full screen on phones. Under the start message a box
  says "N replies · last reply 5 min ago". Active threads are indented
  under their channel in the sidebar; the channel header's thread button
  lists active and archived threads. The quick switcher finds active
  threads; search results say "#channel › thread". The thread's "⋯" menu
  and a right-click in the sidebar: follow, archive, rename, delete,
  mute and notifications.
- Mumble clients never see threads: no mirror, no relay, no history on
  entering the channel.
- Manual check (headless browser): made a thread from a message (name
  from the text), wrote in it with a mention; the other user got it as
  unread with a mention badge, opened it from the reply box, could not
  archive or delete it. Archived it (gone from the sidebar, in the thread
  list), a new message opened it again. Quick switcher and search opened
  it and jumped to the message; phone width shows it full screen.
  Deleting it closed it for both.
- Slow mode: the channel's composer counts the threads we started there
  (their creation time) as turns, as the server does.
- Links to chats the client does not know (maybe an archived thread):
  get-thread asks for it by id, with the history's rights check; the
  thread opens when it comes, at the message. Refused (no thread, or not
  for us: the same answer) says "You cannot open this message here". A
  reconnect mounts a new Client: the place now has the open thread, which
  opens again (asked for if archived).
- Manual check (headless browser): a thread's menu shows "Same as the
  channel" only with an own setting. A link to a message in an archived
  thread opened the thread after a page load; a server restart kept the
  archived thread open. Not checked by hand: channel mute in the sidebar
  (agent-browser sends no right-click) and the slow mode countdown (needs
  staff); both have unit tests.
- Known gaps: no pins panel inside a thread (pinning works). Both panes
  share the upload error. Each channels event and ready queries the
  threads per connection. Thread settings stored before this change with
  a mute only count as own settings in the client, but are not loud for
  push. A thread started and then deleted within the slow mode time no
  longer counts in the countdown (the server still refuses the post).
  Push subscriptions made before this change keep threads of "nothing"
  channels quiet until the app sends its settings again (at its start).

- Known gaps: no pins panel inside a thread (pinning works). A thread's
  mute and notification level do not follow its channel's. Opening a
  link to an archived thread the client does not know yet does nothing;
  a reconnect closes an open archived thread. The composer's slow mode
  countdown does not know that starting a thread took a turn (the server
  refuses the next post). Both panes share the upload error. Each
  channels event and ready queries the threads per connection.

## Media dimensions (2026-10-20)

- Uploads keep width, height (pixels, as shown) and duration (ms) in
  three new columns (0: unknown). internal/mediasize reads them from
  headers only: pictures with image.DecodeConfig (same limits as
  imagesize; JPEG Exif orientation 5 to 8 swaps them, as browsers do),
  MP4/M4A from moov (mvhd duration/timescale, the first tkhd with a
  size; a matrix turned by about 90 or 270 degrees swaps), WebM from
  Segment → Info (Duration × TimecodeScale) and Tracks (PixelWidth/Height,
  DisplayWidth/Height in pixels or as aspect ratio). It walks with
  ReaderAt and one budget of 4096 box/element headers per file, so moov
  at the end of a large file costs a few reads and hostile input cannot
  loop. Truncation tests and fuzz targets (FuzzMP4, FuzzWebM) cover it.
  MP3, Ogg, WAV and FLAC get nothing (yet).
- Old uploads: measured once at start (store.MeasureUploads, flag
  upload_sizes in store_meta), with the type as served (old
  octet-stream uploads with a media name). Files are read with no
  connection held (SQLite has one); results and flag go in one
  transaction. A file missing then is not measured again.
- The upload check for oversized images now uses the same read: an
  inline image type without a size becomes a download.
- Wire: upload objects carry width/height/duration, link previews
  imageWidth/imageHeight (when known; the card stays 80x80).
- Web: imageBox scales the size into 400x300 (the CSS maximum); the img
  gets that width and `aspect-ratio: auto w / h`, so the loaded
  picture's own ratio still wins (object-fit: contain, so a wrong size,
  say Exif in PNG/WebP, does not squash it). Videos get `aspect-ratio: w / h`;
  checked headless that the box is the same before and after loading.
  Players show the server's length until the metadata loads.
- Known gaps: WebM DisplayUnit 1/2 (cm, inches) and PixelCrop are
  ignored (pixel size used); the first MP4 track with a tkhd size wins
  (no hdlr check); JPEG fill bytes before APP1 end the Exif scan (then
  "not turned").

## Media previews (2026-10-20)

- Optional ffmpeg (internal/thumbs). At start: MEDIA_PREVIEWS=0 → off;
  else FFMPEG_PATH or `ffmpeg` on PATH, and `ffmpeg -encoders` picks WebP
  (libwebp) or JPEG (mjpeg). The log says which. No ffmpeg: as before.
  Homebrew's ffmpeg has no libwebp (JPEG path), Alpine's has it.
- Videos (MP4, WebM): a frame at 1 s (the first frame if shorter than 2 s
  or of unknown length; `-ss` past the end makes ffmpeg fail), at most
  480 px. Pictures (PNG, JPEG, WebP) over 1600 px a side or over 1 MB:
  1000 px. GIFs: none (they keep their animation). PNG/WebP only with
  WebP output (yuva420p keeps transparency); JPEG output would lose it. A
  picture's preview not smaller than the picture is dropped. Nothing
  above 2^26 pixels (ffmpeg's memory).
- JPEG Exif orientation: ffmpeg gets `-noautorotate` and the turn as a
  filter (transpose/flip, from mediasize.ExifOrientation), so the preview
  looks as browsers show the original. Videos keep ffmpeg's autorotate
  (display matrix). The real-ffmpeg test checks both.
- Safety: a separate process (exec.CommandContext, 20 s, killed after,
  WaitDelay 1 s), behind `nice -n 10` if there is one, env only PATH,
  `-nostdin -protocol_whitelist file -threads 1`, input `file:<path>`
  with the demuxer forced by the stored type (mov, matroska, png_pipe,
  jpeg_pipe, webp_pipe): no HLS/concat playlist can pull in other files.
  stderr kept to 1 KB, output at most 8 MB. At most 2 processes (one
  semaphore for the queue workers and the backfill); queue of 64, full →
  dropped (the backfill gets it at the next start).
- Files: ffmpeg writes into os.TempDir (the /tmp tmpfs in the read-only
  container), the result is copied into the upload dir as `.preview-*`
  and renamed to `<key>.preview` (rename alone fails across file
  systems). Never in the upload request: queued after the upload is
  stored (browser uploads and Mumble pictures).
- Store: uploads.preview: "" not tried, the type when made, "-" when the
  file gives none (thumbs.ErrFailed: ffmpeg failed, timed out, or wrote
  nothing usable). Other errors (temp dir, disk, shutdown) leave "" and
  the next start tries again; jobs stopped by the shutdown are not
  reported.
- Serving: `/files/<key>/<name>?preview`, the same lookup as the upload
  (unguessable key, plainKey), only if preview is an image type; nosniff,
  sandbox CSP, immutable cache. Deleting an upload (message deleted,
  unused) deletes the preview; a preview done after the upload was
  deleted is removed (SetUploadPreview finds no row).
- Wire: upload objects get `preview` (the URL) once made. When it is made
  after the message was sent, the hub sends the message again as
  `message-edited` (no `edited` mark), so clients swap it in. Chosen over
  clients probing for the URL (404s, and the client would have to know
  the rules).
- Old uploads: a background backfill at start (attached pictures and
  videos with preview "", the newest first, one at a time through the
  same limit). Chosen over making them on request: no 404-then-retry in
  the client, and the rate is bounded by the semaphore.
- Web: lib/media chatSrc (the preview in the chat, the original in the
  lightbox and links; a failing preview falls back to the original) and
  videoPreload (poster → preload="none"). The video src stays the same
  when a poster arrives later, so a playing video is not reloaded. Checked
  headless against a local server (homebrew ffmpeg, JPEG): a 2400x1200
  JPEG shows `?preview` in the chat and the original in the lightbox; a
  3 s MP4 gets a poster, preload none, length 0:03, the same box before
  and after play.
- Docker: `apk add ffmpeg` (ffmpeg 6.1 with libwebp): image 60 MB → 169 MB.
  thumbs tests run in the image read-only with tmpfs /tmp and
  no-new-privileges, as user kallisti: all pass (WebP, alpha kept).
- Known gaps: an APNG (and an animated WebP, where ffmpeg decodes it)
  gets a still preview; the "edited" message can in a narrow window
  arrive before the "message" (then no preview until reload); the resend
  makes clients fetch the link preview again; preview files do not count
  toward the quota; a crash can leave `.preview-*` / `kallisti-preview-*`
  temp files; refused small pictures are scanned at each start (one
  query); non-square pixels (SAR) are not corrected in video thumbnails;
  old octet-stream uploads with media names get none. The systemd unit's
  MemoryDenyWriteExecute applies to ffmpeg too (not checked).

## Save feedback (2026-10-20)

- One pattern for controls that save on the server: `lib/saveFeedback`
  (pure: idle → pending → saved | error, and which event confirms which
  request), `createSave` (runes), `SaveStatus` ("Saved" for 2 s, or the
  error, next to the control). Buttons show "Saving…" and are disabled.
- `connection.request` / `ask` / `sendAwait` return a promise: settled by
  the confirming event, a manage error (now at the waiting control, not
  the toast) or a Mumble PermissionDenied, or "No answer from the server"
  after 5 s. A checkbox or select goes back to the server's value then.
- The server sends no request ids: answers are matched in order. A refusal
  goes to the oldest waiting control; an event confirms the oldest request
  it matches. Mumble requests (bans, registered users) are done when the
  state holds the change (asked for again after the change).
- Dialogs (channel, members, moderators) now close when the server
  confirms, not at once; Mumble's own channel edits still have no answer.
- Not changed: user menu role changes, kick/ban/time-out dialogs, channel
  recording (they close at once; the result shows in the lists; errors
  stay toasts); the status dialog (errors come as chat errors); the bio
  (saved on blur, no status). Known gaps: a refusal of a request sent
  without waiting (list loads) can go to a waiting control; a broadcast
  from another admin can confirm early; an answer after the dialog closed
  is lost; a plain (non-manage) error makes the control wait 5 s.

## Background disconnects (2026-10-20)

### Android and web (part 3)

- On Android, in voice, the voice mix plays through an `<audio>` element
  (MediaStreamAudioDestinationNode → srcObject) with a Media Session
  ("Voice: <channel>", the server name, the app icons). Pages that play
  media are not frozen in the background. lib/voice/output.ts: one output
  bus per AudioContext (it outlives reconnects); lib/mediaSession.ts:
  the pure session data.
- Only on Android (user agent): desktops do not freeze pages, the element
  path adds a little latency there, and with a Media Session the media
  keys (play/pause meant for another player) would leave voice. iOS has
  known trouble with stream-fed `<audio>` while the microphone is open,
  and suspends PWAs anyway.
- No double output, no silence: the bus goes to the speakers until the
  element fires `playing`, and back on `pause`. A refused play() keeps the
  direct path; it is tried again on the next touch. Output device:
  setSinkId on the context and the element. Per-user volume, deafen, the
  music buffer and the limiter sit before the bus; the echo test, stream
  audio and the sounds keep their own direct paths.
- Actions: pause, stop and hangup leave voice (Kallisti servers); plain
  Mumble servers have no place outside voice, so they deafen there.
  playbackState stays "playing" (a "paused" notification could be swiped
  away, and that pauses the element).
- Rejoin: Client already restored mute/deafen first and then the channel.
  Gap fixed: until the server confirms the rejoin (or 5 s pass), the old
  channel counts as ours. Before, the new client reported "not in voice"
  at once, so a second drop during the rejoin forgot the channel. The
  media session stays across the client swap: the old client's teardown
  releases it after 1 s, and the new one takes it over before that.
- No wake lock: the goal is calls with the screen off.
- Checked headless (Chromium, Android user agent stubbed, a warmload
  speaker in General): joining plays the element (live audio track),
  sources keep starting, the bus is off the speakers, metadata and
  "playing" set; leaving pauses it and clears the session. A server
  restart: rejoined General muted, the element never paused. Desktop user
  agent: no element, the direct path as before.
- Not tested: output.ts has no unit tests (DOM and Web Audio glue; the
  decisions are in the tested pure parts). Not checked on a phone: the
  media notification and keep-alive with the screen off, echo
  cancellation on speakerphone, and the latency of the element path.

## Background disconnects (2026-10-20)

- Cause: the bridge pinged the browser every 10 s and closed it after 30 s
  without a pong. A suspended page (macOS app in the background, Android)
  cannot answer. Now bridge.Options.PingGrace (default 2 min): each ping
  waits up to the next one, the last pong time decides. Mumble pings go
  upstream every 10 s as before.
- A page that does not read: the bridge's writes to it block (no
  timeout), so it stops reading upstream. mumbled now gives web clients a
  3 min write timeout (was 10 s, the bridge closes the pipe when it gives
  up); voice is dropped, other frames queue (512, a full queue still
  disconnects). Sidecar text overflows its outbox and resyncs. A ping
  frame write can still time out (5 s, coder/websocket) if the socket is
  full at exactly that moment; data writes normally hold the lock then.
  Murmur upstream: not checked how long Murmur waits for us.
- macOS app: window config backgroundThrottling "disabled" (Tauri sets
  WKPreferences.inactiveSchedulingPolicy to none, macOS 14+) and
  background::no_app_nap (NSProcessInfo activity, user initiated, idle
  system sleep allowed) for the app's lifetime. Compiled and unit-tested
  on macOS and Linux (podman); not run on the dev Mac.

## Desktop links and video (2026-10-20)

- Links: wry answers no new-window request unless the app sets a
  handler. So target="_blank" did nothing in WKWebView (macOS) and
  WebKitGTK (Linux); WebView2 (Windows) opened its own bare browser
  window. Links without a target (the file card) navigated the app
  window itself (an MP4 that WebKitGTK could not play: nothing visible).
- Fix: lib/externalLinks.ts. In the app, a click or middle click on an
  http(s) link to another origin opens the system browser
  (tauri-plugin-opener, capability: open-url for http and https only).
  The listener is on the document and runs after the app's handlers, so
  plain clicks on channel links, message links to this server and images
  (lightbox) stay in the app (they call preventDefault); modifier and
  middle clicks open the browser, like a new tab. The players' download
  buttons now open the file in the browser too (cross-origin: the
  download attribute does not count; media is served inline). The plugin's own click script is
  off (it also takes mailto: and tel:). The recording download link goes
  into the document for its click, so it is caught too.
- Linux video: WebKitGTK plays media with GStreamer. The AppImage had
  plugins-base and -good only: H.264 and AAC gave MEDIA_ERR_SRC_NOT_SUPPORTED
  (canPlayType ""), the client showed the file card. VP9/Opus WebM
  played already. Tauri's linuxdeploy GStreamer plugin copies the whole
  system plugin directory, so CI now also installs gstreamer1.0-libav.
  plugins-bad is not necessary (no h264parse needed for progressive MP4).
- Checked in podman (Ubuntu 22.04 arm64, WebKitGTK 2.50.4, Xvfb), the
  real AppImage with a probe page as its frontend (same binary, other
  frontendDist): before, H.264/AAC MP4 and M4A error 4; after, canplay,
  currentTime 2.9 s after 3 s, ended at 5 s (a 5 s H.264+AAC clip and the
  mediasize fixtures). xdotool clicks on a target="_blank" link, a plain
  link to another origin and a middle click reached a fake xdg-open.
- Size (arm64 AppImage): 108.1 MB before, 150.3 MB after (+42 MB:
  libavcodec, libavfilter and their codec libraries). Ubuntu's FFmpeg is
  built with GPL parts (x264, x265), so the AppImage now carries GPL
  libraries; H.264/AAC decoding can need patent licenses in some
  countries, as for the Linux distributions that ship it.
- Not done: the .deb and .rpm use the system GStreamer; they do not
  depend on gst-libav. macOS and Windows not run (same wry code paths).

## Multiple sessions (2026-10-20)

- mumbled: a registered account (also a guest account) may connect up to
  10 times (`maxSessionsPerUser`). At most one session is in a channel,
  the voice session, the one others see. The others are text-only: in
  `TextOnly` (`NoChannel - 1`), which is no channel. No client has it in
  `seen`, so the existing broadcasts leave them out without new checks;
  the session itself always gets its own UserState (`client.sees`). They
  get the channel tree and the users, no voice in or out (also not by
  whisper to their session, no listening), and others cannot move them.
  Unregistered guests keep one session (findGhost, as before).
- A new web session joins text-only when the account has a voice
  session. A text-only session that joins a channel takes voice: the
  voice session moves to `TextOnly` (its own client sees itself there,
  others see it leave and the other join) and the server mute, deafen
  and priority go along, so a second tab cannot shake off a mute. When
  the voice session disconnects, the newest text-only session goes to
  root, but only where root is a lobby (elsewhere root has voice, and the
  tab would open its microphone).
- Texts written in a text-only tab reach Mumble clients from the voice
  session (they do not know the text-only one, and showed "Server");
  without a voice session the text starts with the name. When the voice
  session leaves, the others keep its server mute.
- A Mumble client cannot be text-only: it takes voice when it connects
  (and the channel of the web session), a second Mumble client replaces
  the first as before, and a web session that joins voice kicks it ("You
  joined voice on another device"). So at most one non-web session.
- Hooks see `TextOnly` as `NoChannel`: a text-only session joins and
  leaves from NoChannel to NoChannel (the sidecar updates the online set
  then), taking voice is a join and losing it a leave (streams end, as
  when leaving). `UserChannel` has none for it, so it can neither stream
  nor watch; it may still read a voice channel's chat it may enter.
- `Online()` has one session per user: the voice session, else the
  oldest. It counts text-only sessions, so pushes stay off while any
  session is connected.
- Moderation: a kick or ban removes every session of the account; a ban
  also covers the certificates and addresses of the other sessions (a
  Mumble client logging in with the account's password may have its own
  certificate). One entry per session at most, without what the ban
  already has (full addresses, as single bans). A time-out
  mutes every session. Channel time-outs, message rules and slow mode
  were per user id already.
- MaxUsers counts users in channels only; more sessions of an account
  that is in already are not refused.
- Sidecar statuses: still by session (clients look them up by the
  session they see), but each session of an account shows the account's
  status, idle only while all its sessions are away (as Discord). A
  status is sent again for every session whose shown status changed
  (`Conn.sent`). Typing no longer goes to the typist's other tabs.
- Web: `isTextOnly`, `voiceElsewhere` and `listedUsers` in the protocol
  package. A text-only tab shows root, is not in voice (the microphone
  stays closed), lists the account once, and the voice bar says "In
  voice on another device" with "Join voice here" (the channel the other
  device is in). Losing voice to another tab leaves the voice chat for a
  text channel. Device names are not shown: the bridge does not tell
  them, and the Mumble state alone (another session with our user id in
  a channel) is enough for the bar.
- Notifications: of several tabs of one account in one browser, only the
  tab used last (focus or shown) shows desktop notifications, with its
  own rules (lib/notifyLeader.ts, a BroadcastChannel per server and
  account). A tab that ends without "bye" (a crash) keeps the others
  quiet until one is used again. Other devices notify as before.
- Manual check (headless Chromium, fake microphone): two sessions of
  alice and one of bob. Both alice tabs connected, bob listed alice once.
  Voice on the first tab: the second showed "In voice on another
  device"; "Join voice here" moved voice, the first tab showed the bar,
  bob still saw one alice and heard the active tab (speaking ring), the
  text-only tab heard nothing. Closing the voice tab brought the other
  into the lobby, still online for bob.
- Read marks sync live: a stored mark (a web tab's mark-read, or a
  Mumble client reading a chat or a direct message) goes as
  `{"type":"read","channel","message"}` to the account's web sessions
  (`Hub.setReadMark`); the client raises `reads`, so unread marks,
  mention counts and the title follow. The sender gets it too: a tab
  re-sends marks above its `reads` on every event (for reconnects), so
  without the echo two tabs ahead in different chats sent theirs to each
  other forever.
- Gap: the connections view lists every session but does not mark
  text-only ones.

## Review 3: server races (2026-10-20)

- A password change saves with `store.ChangePassword`: one transaction
  that updates only while the session is live and the hash is still the
  one checked. A request stalled before its body loses to the owner's
  change (401), and it does not end the owner's session.
- `may` and `userMay` fail closed for a text channel that is gone
  (`textMirror`): mirror 0 means public only for a channel that exists.
  A chat found before its channel was deleted reaches nobody.
- `manage` authorizes with `manageMu` held: a channel moved out of a
  category meanwhile is checked in its new category.
- `NewMessage.Thread`: `AddMessage` checks the thread row in its
  transaction. A thread deleted since the post found it refuses the
  post, and its upload stays unused (for the cleanup).
- Gap: a post to a text channel that staff delete at the same time is
  still stored (hidden channel, comes back if staff restore it).

## Review 3: voice sessions and bans (2026-10-20)

- Voice handoff keeps the old voice session's channel (`State.held`)
  until the new session is in: a Mumble client or tab that took voice in
  the account's temporary channel joined a deleted channel (later a nil
  dereference on a permission refresh). `takeVoiceLocked` returns the
  release; `enterLocked` is the one way into a channel for a session,
  also when it makes a temporary channel (single voice session, server
  mute goes along).
- Slow mode: `MumbleClient` returns who a web session is too (ok: false),
  so binary web texts take the account's turn, not the session's.
- A moderator's move from one moderated channel into another counts the
  moved channels against the destination's 50.
- Bans: a Mumble client's own certificate is banned along with its
  account's; the list is published under the server lock, and `join`
  checks bans again, so a connection admitted meanwhile does not stay.

## Review 3: client state (2026-10-20)

- Push: the opt-out count is in localStorage (`pushOffs`), so an
  opt-out in another tab stops a sync that is running here.
- Reconnect: `Place.session` is the session that held our voice. After a
  reconnect we take voice back only from that old session (the server
  has not let it go yet), not from another device that joined meanwhile.
- Read marks: only a visible tab marks new messages read (`autoSeen`); a
  tab that becomes visible catches up.
- Streams: a watch stopped before the stream socket opens asks for
  nothing (the viewer entry is checked after the await).
- Resync: the client answers `resync` with a `resync` request; the
  sidecar sends a fresh ready (`readyFor`, `resendReady`), with nothing
  told to others. So follows, threads, read marks and the rest come back,
  not only history. A ready keeps known archived threads, as a channel
  list does.
- Save feedback: a channel change is confirmed only by a list with that
  change (names compared as the server writes them, without case and
  marks: Go and JS lower case differently); a privacy change by the
  private list. Before, the privacy change's own "channels" event
  confirmed a rename in the same save.
- Drafts live in `Client` (`Drafts`), so they survive panes that close
  (a thread over the channel on phones, a closed thread pane). Checked
  headless on desktop and phone sizes.

## Review 3: media, streams and CI (2026-10-20)

- WHIP: offers still coming count against MaxSessions on their own and
  must arrive within 10 s (a read deadline through
  http.ResponseController); a new stream key ends the reads of that
  account. An error answer sets "Connection: close": else net/http reads
  the rest of a Content-Length body before it answers, without any
  deadline (the tests with io.Pipe send chunked bodies and did not show
  it).
- Stream thumbnails: Speak is checked again on each accepted thumbnail
  (as for frames). The server decodes the JPEG header: JPEG only, at most
  1024 pixels a side. Clients make them 320 wide and now at most 1024
  high (a narrow, tall window).
- Previews: ffmpeg gets `-max_pixels` (imagesize.MaxPixels), so the coded
  frames are checked, not only the container's display size. A full disk
  (ffmpeg's stderr) or an ffmpeg that does not start is no longer
  ErrFailed, so no NoPreview mark; the next start tries again.
- Recording: the silence before a packet must fit in the bytes left
  (512 per second, as for the final padding), else the recording stops
  at the size limit without writing it.
- RPM: depends on gstreamer1-plugin-libav (Fedora 43 and 44, standard
  repos). Fedora builds it on ffmpeg-free: AAC decodes, H.264 only through
  OpenH264 (Cisco repository); full codecs need RPM Fusion's
  libavcodec-freeworld. Not tested on Fedora with the app.
- CI: test.yml runs `cargo test --locked` for the desktop app on Ubuntu
  22.04 (an empty apps/web/dist is enough for generate_context). It
  passed in a Linux container; the Murmur tests skip without
  KALLISTI_TEST_MURMUR.

## Review 4 fixes (2026-10-20)

- Drafts live in App.svelte and go to Client as a prop: a reconnect makes
  a new Client (`{#key connection}`), and the drafts stay. A new login
  starts without them.
- A password change or "log out everywhere" also kicks the Mumble clients
  that logged in with the account's password: they have the account's
  certificate hash (see identifyAccount), web sessions are left to the
  bridge. Certificate-registered Mumble users are not accounts and stay.
  A login that checks the old password just before the change can still
  join just after the kick (not fixed: rare).
- Combined channel saves compare names with toUpperCase only: the server
  keeps accents ("cafe" and "café" are two names). A rename to a Turkish
  dotted İ now gets no confirmation (JS lowers it to i + U+0307).
- Native history catch-up for a text mirror checks Sees at delivery: move
  hooks arrive late.
- Small request bodies get a 10 s read deadline (config.LimitBody): auth
  POSTs, upload posts until the token is checked, and WHIP offers from the
  start of the request. Production's Caddy passes slow bodies on without a
  timeout. The deadline also ends net/http's wait for the unread rest of
  a body after an early refusal.

## Stale media playback (2026-10-20)

- Attachment players stop and drop their source when they unmount
  (`releaseMedia`, as `{@attach releaseOnDestroy}`). Before, the last one
  that played stayed the media-session target after a reconnect, and a
  media key or headset resumed it hours later.
- Stream sound is dropped while the AudioContext is not "running"
  (suspended, WebKit "interrupted"): its clock stands still, so chunks
  piled up (about 690 MB per hour) and played as one burst on resume. The
  playout re-anchors on the first item after the clock runs again.
- Checked in headless Chromium: a draft stays after the server restarts
  and the client re-mounts.

## Hide a shown spoiler again

- A second click (or Enter/Space) on a shown text spoiler hides it again (`toggleSpoiler` in lib/markdown). A link in a shown spoiler opens instead, and a click that ends a text selection does nothing. Discord keeps spoilers shown; users asked for a way back.
- Shown image and video spoilers get a small "Hide" button at the top right. The image itself still opens the viewer.

## Self-mute survives a refresh

- Self-mute and deafen are saved per server in localStorage ("muted", "deaf" or "") and sent right after connecting, before any voice join, as Mumble keeps them across restarts. A reconnect still uses the in-memory place. Saving starts only after the restore, since a new session starts unmuted. Checked headless: mute, deafen and unmute each survive a reload.

## Personal emoji

- A message or reaction refers to a personal emoji as `<:name:id>`, where id is the image (as Discord does). Markdown runs on escaped text, so the client keeps `&lt;:name:id&gt;` away from emphasis and adds the image after sanitizing, as for server emoji. Mumble clients get `:name:` (mumbleText, also in reply quotes and history). The user types `:name:`. The client sends the reference (toRefs) and changes it back for an edit. Server emoji keep their names: a personal emoji cannot get a server emoji's name.
- Images are stored once per sha256 (`emoji_images`). A user's list is `user_emoji` (name, image id). Images stay when nobody has them, because messages show them. Limits: 50 emoji per user, and 50 new images per user in 24 hours (a user who deletes and uploads again cannot fill the disk). New images follow the message rules for uploads from new accounts. Adding an image from a message is not a new upload.
- A reaction is stored under the image's first name. Then everyone's reactions with the same image are one chip, whatever names they gave it.
- Moderator removal keeps the image row without data. The same row blocks the hash and keeps the name for reactions. Clients get the removed ids (ready, `removed-emoji`) and show `:name:` text. Personal images are `Cache-Control: private`, so shared caches drop them, and a browser's cached copy is not shown. The block only stops the same bytes: a changed image gets through.
- Using an emoji does not require it in your list on the server side: anyone can post `<:name:id>`. The images are public anyway (like server emoji), and moderators can remove one.
- Checked headless (agent-browser, media devices stubbed): add an emoji in the settings, use it with a `:bl` suggestion, a second user sees it and adds it from the message under another name, reacts with it from the picker (one chip), SuperUser removes it for everyone (text in messages and reactions, 404 for the image), and an upload of the same image is refused. Guests have no "My emoji".

## Fediverse login

- "Log in with the fediverse" (Mastodon API: Mastodon, Pleroma/Akkoma, GoToSocial, ...): the server registers an app per instance (cached in `fedi_apps`), sends the browser to the instance's OAuth page (authorization code, PKCE S256) and reads `verify_credentials` at `/api/fedi/callback`. A remote account is keyed by host + remote id; the handle (`@user@host`, host in ASCII) is only for display and follows renames.
- Scope: apps are registered with `read:accounts profile`; servers that refuse `profile` (Mastodon before 4.3) get `read:accounts`. An `invalid_scope` from the authorize page retries once with `read:accounts`. The token is not kept, and not revoked: Mastodon deletes apps without tokens after a day. An app without a login yet is registered again after an hour for the same reason.
- State: a random state in an HttpOnly, SameSite=Lax cookie (path /api/fedi), compared with the callback's state, used once, 10 minutes. Flows and sign-ups live in memory (bounded); a restart ends them. The callback always redirects to the public origin (first ALLOWED_ORIGINS entry); no `*` origin, no fediverse login. Results come back as `?fedi=register|linked|error&reason=<code>`; the client shows only its own texts for known codes.
- Register: an unlinked remote account gets a "choose your name" step (the remote user name, cleaned to valid names). Same rules as other new accounts: name checks and the browser check (captcha) when admins turned it on. A fediverse login is little proof (anyone can run an instance), and the check costs a second or two once. Message rules for new accounts apply by account age, as before.
- Link: needs the current password if the account has one (a stolen session must not add a lasting way in), and the same session at the callback. A guest account that links becomes a normal one. One remote account per account. Unlink is refused while it is the only way to log in; a password-less account sets its first password without an old one (also for Mumble apps). Password logins to password-less accounts are refused (with the dummy hash, same timing).
- First password of a fediverse-only account: needs a fresh login with the linked fediverse account ("Confirm with <host>", `purpose: "reauth"`), in the same session; the callback checks host + remote id and keeps a proof for 5 minutes, used once. Without it `/api/password` answers 403 `code: "reauth"`. A stolen session can no longer set a password and lock the owner out. With fediverse logins turned off, no check (no other way back in). Checked headless (with a `navigator.mediaDevices` stub for the settings dialog).
- SSRF: one guarded client (`fedi.NewClient`, `netguard` shared with link previews): the dialer checks the address really dialed (DNS and redirects included), public addresses on port 443 only, HTTPS only, 3 redirects, 10 s, 64 KB bodies, no proxy. `FEDI_INSECURE_DEV_INSTANCES=1` (dev only, off by default) allows local and plain-HTTP instances, for `server/cmd/fakefedi`. `FEDI_LOGIN=0` turns the feature off.
- Verified handle: opt-in per account; `GET /api/fedi/handle?name=` gives it only when on, and the user card shows it with a check mark.
- Avatar: a fediverse sign-up takes over the remote avatar (`avatar_static`, else `avatar`; Mastodon's `missing.png` and Pleroma's `avi.png` defaults are skipped); a link only if the account has none. Downloaded after the answer (5 s, 2 MiB) through the guarded client, header checked (8192 px a side, 16 Mi pixels) before decoding in pure Go (`avatarimg`: JPEG, PNG, GIF first frame, WebP), then the middle square at 128 px, JPEG q85 on #2b2d31, as the web client's `avatarEncode`. `mumbled.SetAccountAvatar` registers an account that never visited (as its first visit would) and updates online sessions at once. Failures only log. EXIF orientation is ignored (instances strip it from avatars).
- Checked headless against the fake instance: register (with a name clash), log in again, link a password account (wrong password refused), log in through the link, unlink refused for a password-less account, handle hidden until opted in, error notice on the login screen. The settings dialog closes the headless Chromium tab (also on main, before this work), so its fediverse section was checked through the API only.

## Long press on a reaction

- On touch screens a long press on a custom-emoji reaction opens its menu ("Add to my emoji", "Remove for everyone…"), as right-click does with a mouse; iOS has no right-click. Elements marked `data-own-press` keep their own long press, so the message sheet does not open as well. Reactions with plain emoji keep opening the message sheet.

## Pending attachment survives a reconnect and a reload

- On the Android PWA the file picker sends the app to the background: the connection drops (the Client is new after the reconnect) or Android kills the page. A picked file now lives per chat in `lib/pendingFiles` (created in App per login, like drafts) and also in IndexedDB, keyed by server origin + Mumble server + user name + chat key. It comes back when its chat opens, also after another chat or a reload; it goes on send or discard. A failed upload keeps it.
- Bounds: only files within the upload limit (50 MB when unknown), the newest 5, 24 h. IndexedDB failures (private mode, quota) only leave the file in memory. Store calls go through one queue, so a delete never overtakes its put.
- The IndexedDB adapter has no unit tests (no fake-indexeddb); the logic is tested with an in-memory store. Checked headless: pick, server restart (reconnect), reload, send.

## Phone: member drawer no longer pushes the chat aside

- Opening the member drawer focused its first button while the drawer was still off-screen to the right; the browser scrolled the `overflow: hidden` layout to show it (280 px), and the chat slid back. The channel drawer sits to the left, where a container cannot scroll, so it never showed this. Fix: `focus({ preventScroll: true })` and `overflow: clip` on the phone layout. Checked headless at 390 px: the layout's scrollLeft stays 0.

## Sending an attachment across a dropped connection; send nonces

- An attachment send used to count as done once the upload worked: the chat message went out on the WebSocket without a check, so a drop right then cleared the file without a message. Now the send waits for the server's answer (`connection.sendConfirmed`, 15 s): our message confirms it; an error, a timeout or a closed connection keeps the file and caption, and the pending row shows "Not sent: <why>".
- Nonces, as in Discord: a `send` can carry a `nonce` (at most 64 bytes). The server keeps it with the message (columns `nonce`, `nonce_author` with a partial unique index; the author is the user id for registered users, else the name) for 24 h. A send with a known nonce stores nothing: the sender gets the stored message again, nobody else does. This check runs before slow mode, so a retry is never refused for its own first try. Only the sending connection sees the nonce on the broadcast. Without a nonce (Mumble clients, old web clients) nothing changes. If the insert fails because another connection stored the same nonce first, that message is the answer.
- The pending store keeps one nonce per picked file (also in IndexedDB), so each retry uses the same one. The client confirms by nonce; with servers that do not echo nonces, it falls back to the upload key in the file URL.
- No double send after a reconnect: `pendingFiles.send` marks the file as sending in the shared store (above the `{#key connection}` re-mount). The new pane shows "Sending…" and does not send it again. When the old send ends, the file goes (success) or stays with its error. Seen headless: the old Client finishes on the new connection, because Svelte props still read the parent's current `connection` after the re-mount.
- Svelte wiring (ChatPane watch and status) has no unit tests. Checked headless with a dropped echo (retry, one message stored), a delayed upload and a closed socket ("Sending…" in the new pane, a second Enter sends nothing, one message).
- Open: any error that is not about channel changes still fails a waiting send, also one from another request, which shows "Not sent" for a stored message (a retry is harmless). Each retry uploads the file again; the unused upload is cleaned up later.
