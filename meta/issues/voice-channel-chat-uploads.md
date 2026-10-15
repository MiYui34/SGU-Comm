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
