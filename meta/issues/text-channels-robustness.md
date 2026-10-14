# Text channels: robustness follow-ups

## Summary

Edge cases from the text channel review.

## Requirements

- A full outbox drops messages silently. Send a "resync" event (the client reloads history) or close the connection.
- When the server cannot follow the Mumble stream, the client never gets `ready` or an error. Send an error event.
- The identity can have an empty name if the session was missing from the UserState list. Do not become ready without a name.
- Mark registered users (UserState.user_id), so that the UI can show whether a name is verified.

## Acceptance Criteria

- Tests cover the resync, the missing-name and the stream-error cases.
