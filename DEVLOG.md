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
