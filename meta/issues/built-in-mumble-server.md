# Built-in Mumble server (replace Murmur)

## Summary

The Go server speaks the Mumble protocol itself, so Murmur is not needed.
Normal Mumble clients still connect as before. Kallisti clients get the
extended features (accounts, text channels, history) as real server data.

## Requirements

- Mumble protocol over TLS and UDP for normal Mumble clients (1.4 and 1.5,
  legacy and protobuf UDP formats).
- The existing WebSocket API for the web and desktop clients works unchanged.
- Accounts, guest names and text channels come from the server's own data,
  not from sniffing Murmur's stream.
- A simpler permission model (close to Discord roles), mapped to Mumble ACLs
  for normal clients.

## Acceptance Criteria

- The official Mumble client and Kallisti connect to the Go server, see the
  same channels and users, and hear each other.
- The deployment runs without the Murmur container.

## Notes

- Our own server, not based on Grumble (see the spike notes).
- Murmur stays the default until the built-in server passes the same tests.
- Later gain: web clients can use datagrams (WebRTC or WebTransport) for
  voice, not the TCP tunnel.
- Middle step if needed: Murmur's Ice authenticator fixes the identity and
  certificate workarounds, but not the rest.

## Sub-issues

- [Mumble server: spike with a minimal Go server](mumble-server-spike.md)
- [Mumble server: test harness with the official client](mumble-server-official-client-harness.md)
- [Mumble server: channels](mumble-server-channels.md)
- [Mumble server: accounts and registration](mumble-server-accounts.md)
- [Mumble server: permissions](mumble-server-permissions.md)
- [Mumble server: moderation](mumble-server-moderation.md)
- [Mumble server: whispers and shouts](mumble-server-voice-targets.md)
- [Mumble server: Mumble 1.4 clients](mumble-server-legacy-clients.md)
- [Mumble server: avatars, comments, descriptions](mumble-server-blobs.md)
- [Mumble server: limits and server list ping](mumble-server-limits.md)
- [Mumble server: integration into the Kallisti server](mumble-server-integration.md)
