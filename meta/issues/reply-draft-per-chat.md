# Reply and draft stay when switching chats

## Summary

Astra #12: one ChatPane for all chats; switching resets only the pending
file. The reply (and the edit, the draft) carry over; the server refuses
the reply id and the typed text is lost. Reset or keep them per chat.

## Acceptance Criteria

- Switching chats clears the reply and the edit; the draft is kept per
  chat (or cleared); test or manual check.
