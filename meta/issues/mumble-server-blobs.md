# Mumble server: avatars, comments, descriptions

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- User texture and comment, channel description; large ones sent as hashes
  and fetched with RequestBlob; stored for registered users.

## Acceptance Criteria

- A comment set by one client shows in another client, also after a
  reconnect of a registered user.

## Result

- Done: comments (plain text), avatars, long descriptions as SHA-1 hashes
  with RequestBlob (each id once, rate limited), stored for registered
  users (also what was set before registering), clearing others' content
  with ResetUserContent.
- Official client test: it gets the hash and shows the full comment with
  "View Comment" (checked on the screenshot).
