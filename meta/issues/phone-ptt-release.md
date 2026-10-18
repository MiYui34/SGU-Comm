# Phone hold-to-talk stays on after a layout change

## Summary

Astra #3: if the Hold to talk button goes away while held (window or
rotation crosses 700 px), the release is lost and the mic keeps sending.

## Acceptance Criteria

- Unmounting while held stops talking; test.
