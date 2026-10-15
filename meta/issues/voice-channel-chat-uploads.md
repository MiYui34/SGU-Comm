# Voice channel chat with uploads

## Summary

The chat of a voice channel is Mumble's channel text: not stored and without
uploads. Discord's voice channels have a full text chat.

## Requirements

- To be decided: store voice channel chat (keyed by the channel id, which
  the built-in server never reuses) and allow uploads there; Mumble clients
  keep seeing the text (uploads as links).

## Acceptance Criteria

- A file can be uploaded in a voice channel's chat and is still there after
  a reload.

## Decision

- Stored, with uploads: each voice channel gets a chat like a text channel;
  native Mumble users still see the text, uploads as links.

## Result

- Done: each voice channel has a stored chat (hidden channel keyed by the
  Mumble channel id) with history and uploads; Enter to read, TextMessage
  to write, live messages filtered per receiver. Web messages reach Mumble
  clients as text (uploads as links); Mumble clients' texts (also to
  trees) are stored. Tested with the official client in podman: both
  directions, upload link, history after a reload.
- Open, small: voice chats have no unread marks; every reader gets all
  live voice chat messages (default ACLs); broadcastVoice asks the Mumble
  server once per connection; upload links use the first allowed origin
  and stay valid like text channel links.
