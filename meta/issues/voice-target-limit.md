# Server: VoiceTarget messages skip the rate limit

## Summary

`TypeVoiceTarget` does not use the control message limiter (review R11).

## Requirements

- Rate-limit VoiceTarget updates (with enough burst for normal clients), before parsing. Voice packets keep their own limit.

## Acceptance Criteria

- A test: with an empty limiter, a VoiceTarget update is not applied.
