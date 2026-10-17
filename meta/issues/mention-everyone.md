# @here and @everyone

## Summary

`@here` (people online now) and `@everyone` (all) mention many people.
Only users with a permission can use them; for others they are plain
text.

## Acceptance Criteria

- Server checks the permission; tests.
- Mentioned users get the mention badge and notification.

## Notes

- Only Kallisti users: Mumble clients cannot use them and are not pinged.
- `@here` counts only for who is online when it is sent. A reload or a
  reconnect before reading drops its badge (the server's summary at login
  counts `@everyone` only).
- Not in direct chats (every message there counts already).
