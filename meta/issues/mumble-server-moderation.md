# Mumble server: moderation

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- Kick, ban (BanList, by certificate hash and IP range, with duration),
  server mute and deafen, priority speaker, comments of others reset.

## Acceptance Criteria

- A banned user cannot connect; a server-muted user is not heard.

## Result

- Done: kick, kick-ban (certificate and/or address, 1.6 options), ban list
  with expiry, IP bans at accept and certificate bans at login, server
  mute/deafen/suppress, priority speaker, recording flag.
- Official client test: muted (not heard), banned (kicked, refused on
  return with the reason).
- Comments and avatars of others (ResetUserContent) belong to the blobs
  issue.
