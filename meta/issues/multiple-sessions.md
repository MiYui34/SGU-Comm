# The same user on several devices

## Summary

Request: be connected with several clients at once. Standard Mumble
allows one session per account (a new one kicks the old). Our built-in
server could allow several: one session has voice, the others are
text-only (not shown in voice channels); joining voice elsewhere moves
voice there ("in voice on another device"). Design first: how Mumble
clients see it, presence, notifications, moderation actions.

## Decisions (2026-10-20)

- Any number of connections per registered account (incl. guest
  accounts); unregistered guests keep one session.
- At most one session is in voice; the others are text-only: hidden
  Mumble sessions in our own server (in no channel, no voice in or out),
  so the web client still gets the channel tree and users.
- Joining voice on another device moves voice there silently; the old
  device becomes text-only and shows "You are in voice on another device"
  with "Join voice here".
- Every browser tab is its own session.
- One entry per user in member lists; presence combined over sessions
  (online > idle > dnd rules as Discord: dnd if any session is dnd? —
  decide in implementation, document); read marks, mentions, DMs already
  server-side; notifications to all text sessions per their own rules.
- Moderation (kick, ban, time-out, mute, move) applies to all sessions of
  the account. A Mumble client that connects takes voice; web sessions
  become text-only.
