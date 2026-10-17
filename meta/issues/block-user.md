# Hide a user's messages (block)

## Summary

Next to the local voice mute: hide a user's text messages for me. Their
messages show as "Message from a blocked user" with a Show button, as in
Discord. Local only (this browser), by user name, like the voice mute.

## Acceptance Criteria

- Unit tests for the setting; manual check in chat, DMs and search.
- No notifications, no mention badge and no mention sound for blocked
  users' messages. (The unread dot of a channel and the server's mention
  count from before the login still count them: the server does not know
  the list.)
