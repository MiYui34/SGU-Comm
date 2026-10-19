# Desktop app hangs (Linux)

## Summary

Report (Linux AppImage with the audio fix): intermittent whole-app hangs,
sometimes ending with "WebProcess didn't exit as expected after the UI
process connection was closed" in the terminal; sometimes stuck for good.
Seen when joining the Music channel (1x) and opening the user Settings
(2x). Related lead: opening the user Settings crashes headless Chromium
(the page goes to about:blank, even with media APIs stubbed).

## Acceptance Criteria

- Reproduce in podman (real app or WebKitGTK, Xvfb) and in headless
  Chromium; find the cause (e.g. a reactive loop, GStreamer/audio
  deadlock, the bundled GStreamer).
- Fix with a test where the cause is in our code.
