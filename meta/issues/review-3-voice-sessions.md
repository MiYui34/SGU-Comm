# Review 3: voice sessions and bans

## Summary

Voice handoff, temporary channels, slow mode identity, subchannel quota and bans have gaps.

## Requirements

- H1: A Mumble client that takes voice from the account's web session in a temporary channel must not join a channel that the handoff deleted. Make takeover, destination choice, admission and temporary-channel cleanup one consistent step. (session.go admission, textonly.go takeVoiceLocked, MoveUser cleanup.)
- M3: Creating a temporary channel from a text-only session goes through the same voice takeover as a normal move (single voice session, inherits server mute/deafen).
- M4: Slow mode counts binary web text messages against the registered account, not the session.
- M5: Moving a channel into another moderated root counts against the destination's subchannel limit (or root-scoped moderators may not move across roots).
- M6: An account ban also records the real TLS certificate hash of connected Mumble clients. A connection admitted after a ban is published is checked again at final admission.

## Acceptance Criteria

- Regression tests for each item fail before and pass after the fix.
- Go race tests pass.
