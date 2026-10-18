# Offline members: follow-ups

## Summary

1. A guest account that becomes a normal account (sets a password) shows
   in other clients' member list only after they reconnect.
2. A new web account shows under Offline for a moment on its first
   connect (the member event comes before its online state).
3. A renamed user keeps the old name on existing direct chats.
4. Users in private channels I cannot see show as Offline. Like Discord,
   show them as online (with their status), without their channel. Mumble
   clients still do not see them.

## Acceptance Criteria

- Tests for each point (server and web logic); manual check in the browser.
