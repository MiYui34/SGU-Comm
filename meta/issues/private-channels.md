# Invite-only channels

## Summary

Private text and voice channels, like Discord's: only members see them
(non-members do not see them at all, also in Mumble clients); moderators
and admins see all, and add and remove members. Decided 2026-10-18: text
and voice; moderators and admins invite; hidden for non-members.

One source of truth: a member group on the Mumble channel (the voice
channel, or the text channel's Mumble mirror), with an ACL that allows
only members (and staff) to see, enter and speak/write. Kallisti checks
text access through it, as it does for voice chats.

## Acceptance Criteria

- Server tests: non-members get no channel state, no messages, history,
  search results, pins, unread counts or mentions of a private channel;
  members and staff do; @everyone reaches only members.
- UI: a Private switch when creating or editing a channel; a Members
  dialog for staff; a lock icon on private channels.
- Manual check with a member, a non-member and a moderator (web) and a
  Mumble client if possible.
