# Threads

## Summary

Like Discord: anyone who may write can start a thread from a message (or
in a forum-like channel): a short-lived side conversation with its own
history, listed under the channel, archived after a time without
activity. Notifications follow the thread (joined when you write or are
mentioned). Not started; design first.

## Decisions (2026-10-20)

- Anyone who may write in a channel starts a thread from a message (name
  from the message, editable); a side panel on desktop, full screen on
  phones; under the start message a box with the reply count and last
  reply time.
- Active threads indented under their channel in the sidebar; archived
  after 3 days without messages (staff can set the time); a new message
  reopens it; archived ones stay readable and searchable.
- Rights as the channel (read, write, moderators, slow mode, message
  rules). No private threads for now.
- Notifications: you follow a thread when you write in it or are
  mentioned; mutable like a channel.
- Mumble clients don't see threads.
- Server: a thread is a chat of its own, linked to its parent channel and
  start message; reuses history, search, read marks, reactions, uploads.
