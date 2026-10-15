# Typing indicator

## Summary

"lain is typing…" over the message box.

## Requirements

- The client sends a short "typing" signal at most every few seconds while
  typing; the server passes it to others in the channel (nothing stored).
- The indicator ends after a few seconds, or when the message arrives.

## Acceptance Criteria

- Sidecar test: passed on only to readers of the channel, rate limited.
- Client tests for the timing; works in the browser.
