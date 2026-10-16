# Streaming: sound, also for Mumble clients

## Requirements

- The shared tab or system sound (where the browser offers it) is part of the stream, in sync with the picture, with its own volume in the player.
- Desktop Mumble clients hear it as a hidden stream user's voice (forwarded Opus, not re-encoded); web clients never see that user.

## Acceptance Criteria

- mumbled tests for the stream user (only for Mumble clients); checked in the browser and with a Mumble client.
