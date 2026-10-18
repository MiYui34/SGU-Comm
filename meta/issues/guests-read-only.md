# Guests read only

## Summary

A server setting (admins, in the server settings): guests may read but
not write, react or upload. Text channels show "Log in or make an account
to write" instead of the composer. Mumble guests get the same (no
TextMessage). Voice stays as it is.

## Acceptance Criteria

- Server tests: with the setting on, guests cannot send, react, upload or
  start direct chats; registered users can; the setting is saved.
- Web: the setting in server settings; the composer note for guests.
