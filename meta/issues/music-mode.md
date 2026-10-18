# Music mode

## Summary

The web client and the desktop app are tuned for speech: 40 kbps mono,
automatic gain always on, a noise filter that cannot be turned off. That
pumps and cuts music. A "Music mode" switch in the voice settings turns
off the noise filter, echo cancellation and automatic gain, and sends
about 128 kbps (the server allows 558 kbps per user), stereo if the
input has two channels. Voice activity still works; push-to-talk too.

## Acceptance Criteria

- Pure functions for the capture settings and encoder config in music
  mode, with tests.
- The switch in the settings; a small note in the voice panel while on.
- Manual check: music through the web client sounds clean on the other
  side (no pumping, no cut notes); the server accepts the bitrate.
