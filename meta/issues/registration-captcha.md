# Captcha on registration

## Summary

Optional, against scripted account and guest creation: a self-hosted
proof-of-work challenge (no third-party service, no tracking), solved by
the browser in a second or two before registration or guest join.
Mumble clients are not affected (they have their own limits).

## Acceptance Criteria

- Server tests: registration and guest join need a valid, unused
  solution when the setting is on; the difficulty can be set.
- Web: the challenge runs without user action and shows progress.
