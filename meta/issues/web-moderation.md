# Moderation in the web app

## Summary

Moderators and admins can kick, ban (with a reason and a time), time out,
server-mute, server-deafen and move users from the user menu. Mumble has
all of it except time-outs.

## Acceptance Criteria

- Each action works against the built-in server and is checked by its
  permissions; tests where there is logic.
- Time-out: no chat and no speaking for a chosen time.
