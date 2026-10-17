# Moderation in the web app

## Summary

Kick, ban (with a time), server mute and deafen, move, and time-out from
the user menu (member list and voice channel users).

## Requirements

- Actions only with the Mumble permission in the right channel; the
  server checks again.
- Kick and ban ask first, with an optional reason; bans for 1 hour,
  1 day, 7 days or forever.
- Time-out (Kallisti): no chat and no speaking for 5 minutes, 1 hour,
  1 day or 1 week; remove early.

## Result

- Kick: Mumble UserRemove. Ban forever: UserRemove with ban. Timed bans:
  sidecar request "ban", the built-in server bans with a duration (same
  rules as UserRemove). Mute, deafen and move: UserState.
- Time-outs: sidecar "time-out" request, built-in server. MuteDeafen in
  root (a time-out is a mute everywhere; MayManage is about channels),
  never on SuperUser, on admins and moderators only by admins (as kick).
  Registered users by user id (saved, survive reconnects and restarts),
  guests by session. While timed out: server-muted (unmute refused), no
  Mumble texts, no sidecar send, react or edit. Direct messages too. A
  timer unmutes at the end (also after a restart); a mute from before the
  time-out stays.
- Web: the user menu shows what the permissions allow (queried for the
  user's channel and the voice channels), "Timed out until ..." for
  everyone, and the timed-out user gets a notice in the message box.
- Timed bans and time-outs need the built-in server; on Murmur only
  forever bans show.

## Open edge cases

- A guest escapes a time-out by reconnecting (keyed by session); use a
  ban.
- A time-out of an offline registered user cannot be removed from the web
  (the request needs a session); it ends by itself.
- The move list uses our Enter permission; the server checks the target's
  Enter (or our Move) and may refuse.
- Timed-out users can still upload files (the message is refused) and
  change their comment or avatar.
