# Mumble server: channels

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- Channel tree stored in SQLite: create, edit (name, description,
  position), remove, links, temporary channels (removed when empty).
- Users move between channels; voice and text go to the user's channel.
- State logic as a pure core with unit tests; the network layer stays thin.

## Acceptance Criteria

- Two clients in different channels do not hear each other; after a move
  they do. Channels survive a restart.
