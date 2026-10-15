# Mentions

## Summary

"@lain hey how are you" should ping lain: highlight the message for them,
and mark the channel when they are not looking at it.

## Requirements

- A message mentions me when it has "@" plus my name (case does not
  matter, whole name only); my own messages never do.
- Mentioning messages are highlighted for me; "@name" of me stands out.
- Channels with unread mentions show a count badge (text channels and
  voice channel chats), cleared when I open the channel.
- A short sound when a mention arrives in a channel I am not looking at.

## Acceptance Criteria

- Unit tests for the detection and counting; browser check with two users.

## Notes

- Later: @ autocomplete, desktop notifications.

## Result

- Done: detection on escaped HTML (whole name, any case, not in URLs,
  addresses or after entities), line and @name highlight, count badges on
  text channels and voice channels (via their chat), a short sound (not
  while deafened, on the output device, at most once a second).
- Tested in the browser with two users: badge, highlight, clearing on
  open, no badge in the open channel, own messages not highlighted. The
  sound is not tested (headless tabs play nothing).
- Limits: mentions in a voice channel chat that is not open (lobby, voice
  off) count for the sound but show no badge; no @ autocomplete yet.
