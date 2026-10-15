# Reconnect automatically

## Requirements

- After an unexpected close: reconnect with growing waits (1 s up to 30 s),
  at once when the tab becomes visible or the network comes back; a
  "Reconnecting…" bar instead of the login screen.
- Not after a kick, ban, refusal or logout.
- Keep the view (text channel, conversation) and rejoin the voice channel;
  keep the audio context (no user gesture needed again).

## Acceptance Criteria

- Unit tests for the decision and the waits; browser check: the server
  restarts and the client comes back in the same channel.
