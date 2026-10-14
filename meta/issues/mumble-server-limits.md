# Mumble server: limits and server list ping

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- Answer the unencrypted UDP server-list ping (users, max users,
  bandwidth); user limit; connections per IP; UDP client index by IP.

## Acceptance Criteria

- The official client's server list shows the server as reachable with the
  user count; connection floods from one IP are refused.
