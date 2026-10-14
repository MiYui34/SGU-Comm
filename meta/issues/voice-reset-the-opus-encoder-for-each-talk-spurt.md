# Voice: reset the Opus encoder for each talk spurt

## Summary

The desktop Mumble client resets the Opus encoder when a new transmission starts. We keep the state, so the first frame carries a little state from the previous spurt.

## Requirements

- Reset the encoder state at the start of a transmission, without the extra packet that flush() adds.

## Acceptance Criteria

- The first packet of a talk spurt does not depend on the previous spurt.
