# Connection info in the web client

## Summary

Like Discord's voice panel: a small signal icon next to "Voice
connected" (green, yellow, red); its popup shows ping (round-trip time on
the connection, from Mumble Ping messages), audio that came too late to
play or was missing (from the jitter buffer), and the transport
(WebSocket or UDP in the desktop app).

## Acceptance Criteria

- Pure functions for the quality level and averages, with tests.
- The icon and popup; values update every few seconds.
- Manual check, also with a slowed connection (browser throttling).
