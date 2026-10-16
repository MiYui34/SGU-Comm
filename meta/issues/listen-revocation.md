# Server: taking away Listen does not stop a listener

## Summary

Listen is checked only when a user starts to listen to a channel. After an ACL change, existing listeners still get the audio (review R3).

## Requirements

- After permission changes, remove listeners that may no longer listen, and tell the clients.

## Acceptance Criteria

- A test: deny Listen; the listener is removed and gets no more audio.
