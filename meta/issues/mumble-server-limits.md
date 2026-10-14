# Mumble server: limits and server list ping

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- Answer the unencrypted UDP server-list ping (users, max users,
  bandwidth); user limit; connections per IP; UDP client index by IP.

## Acceptance Criteria

- The official client's server list shows the server as reachable with the
  user count; connection floods from one IP are refused.

## Result

- Done: unencrypted server-list ping (legacy and protobuf extended), a
  global rate limit for its replies, user limit (SuperUser and reconnects
  exempt), optional connections per IP (IPv6 per /64), a cap on open
  handshakes, clients by IP for UDP.
- Tested with Go tests; the official client's server list is not tested
  (it needs GUI steps to add a favourite).
