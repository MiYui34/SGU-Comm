# Web: keep the microphone open while muted

## Summary

Mute closes the microphone and unmute opens it again. The browser then
reconfigures the audio device, so incoming audio (music) stops for a moment.

## Requirements

- In a voice channel the microphone stays open while muted; nothing is sent.
- A setting "Close the microphone when muted" (off by default) keeps the old
  behavior.

## Acceptance Criteria

- Tests for when the microphone is open, with and without the setting.
- The setting is in the settings dialog and is kept.

## Notes

- Cause not reproduced (the test browser has no real audio device).
