# Voice: microphone stays on when the worklet fails to start

## Summary

In `capture.svelte.ts` `start`, if `workletNode()` fails, the new stream is never stopped. The browser keeps the microphone open (review R13). A failed worklet promise stays cached, so later tries fail the same way.

## Requirements

- Stop the stream's tracks on every error path.
- A failed worklet load can be tried again.

## Acceptance Criteria

- A test with a failing worklet: the tracks are stopped.
