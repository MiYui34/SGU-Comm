# Voice: fall back to the default microphone

## Summary

If the selected microphone is unplugged, or opening it with `exact` fails, the old stream stays on but the dialog shows the new device. There is no fallback and no `ended` handler.

## Requirements

- Handle the track `ended` event and getUserMedia failures by falling back to the default device.
- Show the device that is really in use.

## Acceptance Criteria

- Unplugging the selected microphone switches to the default device, and the UI shows it.
