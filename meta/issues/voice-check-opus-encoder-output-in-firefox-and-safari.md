# Voice: check Opus encoder output in Firefox and Safari

## Summary

The terminator logic expects one encoder output per 20 ms input, as in Chrome. `flush()` is not used, because in Chrome it adds a packet after the terminator (see DEVLOG.md).

## Requirements

- Check the packet sequence in Firefox and Safari: no frame-number gaps or repeats, one terminator per talk spurt.

## Acceptance Criteria

- The listener check (frame numbers and terminators on the wire) passes in Firefox and Safari.
- 2026-10-14: Playwright Firefox and WebKit cannot start in the development sandbox. Needs a manual test, or a CI job with real browsers.
