# Mumble server: accounts and registration

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- Registered users: by certificate hash (Mumble clients) or by password,
  stored in SQLite. Self-registration (UserState user_id 0).
- Registered names are reserved for their owner.
- A reconnect with the same certificate replaces the old session.
- Last channel is kept for registered users.

## Acceptance Criteria

- A registered user gets the same user id after a reconnect; others cannot
  use the name; a reconnect after a network drop works at once.

## Result

- Done: certificate and password login as Murmur, reserved names,
  self-registration, ghost replacement, last channel, server password,
  QueryUsers, login failure limit per IP. Tested with the official client
  (SuperUser login).
- Registering others (Register permission) and UserList moved to the
  permissions issue.
