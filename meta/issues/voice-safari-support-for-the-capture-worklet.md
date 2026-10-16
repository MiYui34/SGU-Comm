# Voice: Safari support for the capture worklet

## Summary

Chrome and Firefox process an AudioWorkletNode with `numberOfOutputs: 0`. Safari might not.

## Requirements

- Test capture in Safari.
- If needed, connect one output through a GainNode with gain 0 to the destination.

## Acceptance Criteria

- Voice capture works in current Safari.
- 2026-10-14: the worklet now has one silent output routed to the destination (commit "route the capture worklet..."). Still needs a test in Safari: no Safari or WebKit can start in the development sandbox.
- 2026-10-16: in Lockdown Mode Safari has no AudioWorkletNode. The app now says which feature is missing. Still needs a test with Lockdown Mode off (it can be turned off per site).
