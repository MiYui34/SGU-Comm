# Mumble server: avatars, comments, descriptions

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- User texture and comment, channel description; large ones sent as hashes
  and fetched with RequestBlob; stored for registered users.

## Acceptance Criteria

- A comment set by one client shows in another client, also after a
  reconnect of a registered user.
