# Slow mode

## Summary

Per text channel: each user may post once per N seconds (off, 5 s, 30 s,
1 min, 5 min, 1 h). Staff are exempt. Set in the channel settings. The
composer shows the time left.

## Acceptance Criteria

- Server tests: the wait is enforced per user and channel; staff exempt;
  the setting is saved and sent to clients.
- Web: the setting; the countdown in the composer.
