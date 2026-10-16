# Listen-only voice channels

## Summary

Some voice channels (for example a music channel with the jukebox) should
be listen-only: only some users may speak. The server supports this with
Mumble ACLs, but the web app cannot set it.

## Requirements

- The voice channel dialog has a "Listen only" switch and a list of the
  users who may still speak (registered users only).
- Admins and moderators can set it (moderators have Write below root).
- Underneath, it is plain Mumble ACL entries in the channel: deny Speak to
  `all`, allow Speak to each listed user. Mumble clients see and can edit
  the same entries.
- Not for root and not for the text channel mirrors.
- All web clients learn which channels are listen-only, also after a change
  from a Mumble client.

## Acceptance Criteria

- A moderator makes a channel listen-only in the web app; others in it are
  suppressed; the listed users can speak.
- Turning it off removes the entries again; other ACL entries stay.
