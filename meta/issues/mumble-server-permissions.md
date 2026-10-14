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

## Acceptance Criteria

- A user without the Enter permission cannot join a channel; an admin can
  edit the ACL with the official client.

## Notes

- A simpler role view (like Discord) for the Kallisti UI can come later, on
  top of the same model.
