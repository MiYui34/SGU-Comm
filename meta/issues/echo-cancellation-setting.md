# Echo cancellation setting

## Summary

On macOS WebKit (desktop app, Safari) echo cancellation uses Apple's voice
processing unit, which puts a Bluetooth headset into its call mode (worse
sound, its own microphone active) even when another microphone is chosen.
Chromium is not affected. A setting "Echo cancellation" (on by default)
can turn it off; with a headset there is no echo.

## Acceptance Criteria

- Unit test for the setting; the microphone reopens when it changes.
- Manual check in the desktop app with a Bluetooth headset (user).
