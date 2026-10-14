# Voice: Safari support for the capture worklet

## Summary

Chrome and Firefox process an AudioWorkletNode with `numberOfOutputs: 0`. Safari might not.

## Requirements

- Test capture in Safari.
- If needed, connect one output through a GainNode with gain 0 to the destination.

## Acceptance Criteria

- Voice capture works in current Safari.
