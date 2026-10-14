# Text channels with persistent history

## Summary

Discord-style text channels, stored by the sidecar. Mumble has no text-only channels, and its in-room chat is not stored. Part of [Sidecar: Discord-style server features](sidecar-discord-style-server-features.md).

## Requirements

- Text channels are sidecar objects, shown next to the Mumble voice channels.
- Messages are stored (SQLite) and loaded when a channel is opened.
- A separate API next to the byte bridge (for example JSON over a second WebSocket channel, or HTTP). The server does not parse the Mumble byte stream for this.
- Voice channels keep their temporary in-room chat.

## Acceptance Criteria

- A message sent to a text channel is visible to a user who joins later.
- Text channels work while the user sits in any voice channel.
