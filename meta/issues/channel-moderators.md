# Moderators for single channels

## Summary

Today moderators moderate the whole server. Admins (and moderators?
decide) can make someone a moderator of one channel (or a channel and
its subchannels): there they may delete and pin messages, set the topic
and slow mode, mute, move and kick from the voice channel, manage the
members of a private channel; nowhere else. Built on Mumble ACLs (a
per-channel group with the rights), so Mumble clients follow the same
rules.

## Acceptance Criteria

- Design first: which rights a channel moderator gets (text and voice),
  how it is shown (a badge in that channel's member list), where it is
  set (channel settings → Moderators).
- Server tests for each right inside and outside the channel; they
  cannot act on server staff.
- Web: the setting and the badge.

## Decisions (2026-10-20)

- Rights in their channel: delete and pin messages, set the topic and slow
  mode; mute, move out of and kick from the voice channel; add and remove
  members and make invite links if the channel is private; time-out in
  that channel only (new: a channel time-out stops writing and speaking
  there, server-wide time-outs stay as they are).
- Appointed by admins and server moderators.
- Scope: the channel and its subchannels (as Mumble ACLs inherit).
- Shown: a badge next to the name only in that channel's member list.
- Never over server staff; nobody acts on SuperUser.
