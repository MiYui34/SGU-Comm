# Text channels for Mumble clients

## Summary

Kallisti text channels are invisible to normal Mumble clients. Mirror
each one as a Mumble channel where nobody may speak, under a "Text
channels" parent; bridge messages both ways.

## Requirements

- Sub-issues:
  - [x] [mumbled: text channel mirrors and user moves](text-channels-mumbled.md)
  - [x] [store: Mumble channel per text channel](text-channels-store.md)
  - [x] [sidecar: sync mirrors, bridge messages, history](text-channels-sidecar.md)
  - [x] [web: hide mirrors from the voice list](text-channels-web.md)

## Acceptance Criteria

- A Mumble client sees "Text channels" with "#general" etc., cannot speak
  there, and can read and write messages that web users see, and the
  other way round.
- On the first visit in a connection a Mumble user gets the last 20
  messages; on later visits only what came while away (at most 20).
- Creating, renaming, ordering and deleting text channels in Kallisti
  changes the mirrors.

## Notes

- A Mumble client is in one channel at a time: reading a text channel
  takes a Mumble user out of voice. Channel listening carries no text.
- The Mumble client keeps its chat log per connection, so "seen" is
  tracked per connection.

## Result

- Done. Tested: mumbled, store, sidecar and web tests; the official
  client (1.5) joins "Text channels/#general", cannot speak, gets the
  history on a new connection and web messages live; the web client hides
  the mirrors.
- Known limits: moves reach the sidecar a little late, so a message
  written at that moment may come twice or count as seen; a user whose
  saved channel is a mirror may get the history before the client is
  ready (only without the lobby); an empty "Text channels" parent shows
  in the web voice list when there are no text channels.
