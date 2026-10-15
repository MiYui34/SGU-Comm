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

## Result

- After an unexpected close of a connection that was in: a bar with
  "Retry now", the old view stays readable (no input), attempts with
  growing waits (at once on a visible tab or the network back), one at a
  time, stopped by logout or a new login, given up after 15 s each; a
  guest's old session holding the name gets a few more tries. The new
  session comes back to the view, the voice channel and the mute state
  (muted before joining voice). Not after a kick, a refusal or a
  certificate question. The audio context survives reconnects.
- Tested: unit tests; browser: server restart, back in the same text
  channel and voice channel, still muted, microphone never opened.
