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

## Decision

- Root is the lobby: no voice there (Speak denied in root), shown as
  "online, not in voice"; a "General" voice channel is made on the first
  start. Same for native Mumble users.

## Result

- Done: root is a lobby on Kallisti servers; the web client starts in a
  text channel, joins voice by click, leaves with the voice bar; the last
  channel is not restored there. The microphone is open only in voice and
  unmuted (or in the settings, without sending). Checked in Chrome with a
  counting fake microphone: 0 tracks in the lobby, muted and after leaving
  or a reload; 1 in voice.
