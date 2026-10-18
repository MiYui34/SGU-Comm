# Undeafen keeps an earlier mute

## Summary

Undeafening always unmutes the microphone, also when the user had muted
it before deafening. Like Discord: undeafening brings back the mute state
from before. (Deafen still mutes, and unmuting while deafened still
undeafens: Mumble has no "deaf but not muted".)

## Acceptance Criteria

- Tests for toggleDeaf with and without an earlier mute.
