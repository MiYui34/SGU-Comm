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
  Mumble client if possible. (Done 2026-10-18 on the web; no Mumble client.)

## Notes (implementation)

- mumbled hides every channel a user may not Traverse (and the channels
  below it), with the users in it: no ChannelState, UserState, comments,
  avatars or descriptions (visibility.go, per-client `seen`; changes of
  access send the difference). Users in a channel that becomes hidden to
  them move up. Voice over links reaches only who sees the channel.
- A private channel: "all" deny Traverse|Enter, then members, moderator
  and admin allow them (ApplySubs: subchannels are private too); the
  channel-local group "members". Text channels use their mirror. Only
  staff (isStaff) change it; Mumble ACL edits are reported too.
- Members are registered user ids; guest accounts are registered, so they
  can be members.
- Limits: a mirror made anew (lost one) is public; per-user-ACL admins
  without the admin group do not see private mirrors; users with Write
  from an ACL entry (e.g. a channel's creator) keep access; denials still
  name hidden channel ids (existence can be guessed); upload URLs are not
  tied to channels.
