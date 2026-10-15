# Moderator role

## Summary

Admins (SuperUser, the admin group) can make registered users moderators.
Moderators can do nearly all that admins can, but cannot make moderators
or admins. Finer roles (per channel) later.

## Requirements

- Built on Mumble ACL groups: a `moderator` group in root, so normal
  Mumble clients see and edit it in the ACL editor.
- Moderators: kick, ban, move, mute, make/edit/delete channels (voice and
  text), register users; no changes to root's ACL and groups (where roles
  live) or to the ban list.
- Those changes need Write in root (admins only).
- Web: admins make and unmake moderators from the user menu (registered
  users only); a badge shows moderators and admins.
- Existing servers get the role once (migration), as the lobby did.

## Acceptance Criteria

- SuperUser makes a web account a moderator; that user can then create,
  edit and delete channels and kick users, but sees no "Make moderator".
- A moderator cannot change root's ACL or groups (server refuses).
- Tests in mumbled, sidecar and web; the official client sees the group.

## Result

- Done as described. Moderators also cannot act on staff (kick, ban,
  rename, unregister); they may still mute, move or reset an admin.
- Below root, Write still allows a channel's ACL (as Murmur): channel
  roles stay in that channel; roles() reads only root.
- Tested: mumbled, sidecar and web tests; official client suite (the ACL
  editor shows @moderator); in the browser, SuperUser made a web account a
  moderator and back, live, without reconnects.
