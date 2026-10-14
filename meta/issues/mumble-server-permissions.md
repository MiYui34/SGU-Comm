# Mumble server: permissions

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- Mumble's ACL model (groups, ACL entries with inherit, per channel) as the
  stored model, so Mumble clients can show and edit it (ACL message).
- PermissionQuery per channel, PermissionDenied on refused actions.
- Every action checks permissions (enter, speak, text, make channel, move,
  mute, kick, ban, register, write ACL).
- A superuser/admin role.
- Registering other users (Register permission) and UserList (list,
  rename, unregister registered users).

## Acceptance Criteria

- A user without the Enter permission cannot join a channel; an admin can
  edit the ACL with the official client.

## Notes

- A simpler role view (like Discord) for the Kallisti UI can come later, on
  top of the same model.

## Result

- Done: Murmur's ACL evaluation and groups, default ACL, checks in all
  handlers, ACL editor message, suppress without Speak (also per linked
  channel), can_enter, permission flushes, registering others, UserList.
- Official client tests: kept out of a channel without Enter; SuperUser
  edits an ACL in the client's editor and the server applies it.
- Small differences to Murmur, open: a UserState with user_id and other
  fields is not checked as a whole before acting; access tokens compare
  case-sensitively and cannot change after login.
