# Cap chat history in memory

## Summary

`state.messages` grows without limit in long sessions.

## Requirements

- Keep at most N messages (for example 1000) in the reducer.

## Acceptance Criteria

- A test shows that older messages are dropped above the cap.
