# Stored direct messages

## Summary

Direct messages are Mumble private messages: only in the browser tab, only while both are online. Between registered users (also temporary guest accounts) they should be stored like channel messages, with history, offline delivery, unread badges and uploads.

## Decisions

- A conversation is a hidden chat row for the pair of user ids (as voice chats): messages, edits, deletes, reactions, uploads, read marks and previews work as in channels.
- Only the two users may read or write it; it never shows in search for others.
- Classic guests (no account) keep live-only Mumble private messages.
- Private messages between desktop Mumble clients (both registered) are stored too.

## Sub-issues

- [Store and sidecar](direct-messages-server.md)
- [Web app](direct-messages-web.md)
- [Mumble clients](direct-messages-mumble.md)
