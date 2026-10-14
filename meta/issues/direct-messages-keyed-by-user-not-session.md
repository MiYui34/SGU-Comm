# Direct messages keyed by user, not session

## Summary

Private messages, unread counts and read state are keyed by session number. Murmur reuses session numbers, so a new user with a reused session inherits the old partner's history and read count.

## Requirements

- Key conversations by a stable identity (user ID for registered users, else name), or clear the state when a session ends.

## Acceptance Criteria

- A test shows that a new user with a reused session number starts with an empty conversation and no unread count.

## Notes

- The open DM view already checks the name, so it does not send to the wrong person.
