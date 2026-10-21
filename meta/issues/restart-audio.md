# Restart audio when entering voice and on device changes

## Summary

A long-open tab keeps the AudioContext made for the output device of that time. When the device changes (a Mac leaves its connected devices, headphones connect), Chrome resamples the old context and music in voice sounds crackly until a refresh.

## Requirements

- Entering voice from outside voice (a click) starts with a fresh AudioContext.
- An output device change while in voice (`devicechange`) restarts audio: new context, playback, capture path and stream audio move to it; a short gap is fine.
- No restart when moving between voice channels.
- Android audio element / media session and the Linux startup workaround keep working; a restart without a user gesture shows the existing sound hint.

## Acceptance Criteria

- Tests for the restart decision and the swap logic; web tests and svelte-check pass; headless check that voice still works after a restart (as far as headless allows).
