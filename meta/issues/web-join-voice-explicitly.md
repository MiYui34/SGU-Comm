# Web: join voice channels explicitly

## Summary

The web client is always in a voice channel and the microphone is always
open. As in Discord, users should start in #general and join a voice channel
on purpose; the microphone is only open while they are in voice and not
muted.

## Requirements

- After login: not in voice; the text channel is shown.
- Join a voice channel by clicking it; leave with a button.
- The microphone stream (getUserMedia) is open only while in voice and not
  muted; the browser's recording indicator goes off otherwise.
- How "not in voice" maps to Mumble (users are always in some channel) is
  to be decided.

## Acceptance Criteria

- A new web session does not open the microphone and is not heard.
- Joining a voice channel opens the microphone; muting or leaving closes it.
