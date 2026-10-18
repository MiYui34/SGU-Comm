# Server crash: a voice target with a deleted channel

## Summary

Found by the Astra review (#1), confirmed with a test: a voice target
(whisper/shout) with linked channels keeps a channel after it is
removed; the next voice packet dereferences the missing channel in
State.linked and the whole server crashes. Temporary channels go away by
themselves, so any registered user with a Mumble client can do it.

## Acceptance Criteria

- Test: target with links to a channel, channel removed, Receivers does
  not panic and finds no one there.
