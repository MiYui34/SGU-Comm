# Playback buffer setting and a music buffer

## Summary

Over WebSocket nothing is lost, but audio can come late; the web client
starts playing 60 ms after a stream starts, so a stall over 60 ms is a
gap. Two changes:

1. A "Playback buffer" setting in the voice settings: Low 60 ms
   (default), Medium 120 ms, High 250 ms, for every speaker. For
   listeners with a bad connection; costs them only delay.
2. Music speakers get about 500 ms automatically (or the setting, if
   larger): who speaks in a listen-only channel, and who sends music
   (stereo or high-bitrate Opus, as music mode does). Delay does not
   matter for one-way music.

The connection info popup suggests a larger buffer when late audio is
high ("Late audio 4 %: try a larger playback buffer").

## Acceptance Criteria

- Pure functions for the start delay of a speaker (setting, listen-only
  channel, music packets) and the max queue (scaled with it), with tests.
- The setting applies live, saved like the other settings.
- Manual check: with Medium/High fewer gaps on a throttled connection;
  the jukebox gets the music buffer.
