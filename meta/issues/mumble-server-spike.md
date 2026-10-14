# Mumble server: spike with a minimal Go server

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md). Find out how
much work the server is.

## Requirements

- A minimal Mumble server in the Go module (behind a flag, Murmur stays the
  default): TLS, Version/Authenticate/ServerSync, one channel, UserState and
  UserRemove, TCP and UDP voice routing, ping.

## Acceptance Criteria

- The official Mumble client connects to the minimal server, and two clients
  hear each other (over UDP).
- A short written result with an estimate for the full server.

## Notes

- Tests can use the existing bridge and protocol tests, which run against a
  real Murmur now.
- Grumble was checked briefly and is not a base (user decision: peripheral).
  Findings: BSD, about 14k lines, 8 commits since 2023, announces 1.2.5,
  legacy UDP only (no 1.5 protobuf format). Its OCB2 output is identical to
  ours. It can be a reference for ACL semantics, nothing more.

## Result

- `server/internal/mumbled` (about 600 lines) and `server/internal/cryptstate`
  (about 250 lines), run with `cmd/mumbled`.
- Passes: its own tests (race detector), the desktop bridge tests written
  against Murmur (UDP voice both ways), and the web client through the Go
  bridge in a browser (voice and text).
- Open: a test with the official Mumble desktop client (needs a window, not
  possible in the development sandbox). This issue stays open for it.
- Known limits for that test: a reconnect after a network drop gets "name in
  use" for up to 30 s (no client certificates yet); private messages are
  dropped; the server list shows the server as unreachable (no answer to the
  unencrypted UDP ping); 1.4 clients are refused.

## Estimate for the full server

The spike is the protocol core. Still to do, by size (S < M < L):

- L: permissions. A role model like Discord's, mapped to Mumble ACLs and
  groups for normal clients (ACL message, PermissionQuery per channel).
- M: channels. Create, edit, remove, move users, temporary channels, links,
  stored in the database.
- M: accounts. Registration with our account table, certificate hashes,
  kick of the old session on reconnect with the same certificate.
- M: moderation. Kick, ban, server mute and deafen, PermissionDenied.
- M: integration into the main server. Web clients join in-process, not over
  TCP to a separate server; text channels become real channels.
- S: whispers and shouts (VoiceTarget), legacy UDP format for 1.4 clients,
  avatars and comments (RequestBlob), server list ping, user and connection
  limits per IP, index of clients by IP for UDP.

Together this is about four to six times the spike. Permissions are the
largest risk, because Mumble clients show and edit ACLs directly.
