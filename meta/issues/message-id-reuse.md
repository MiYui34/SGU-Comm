# Store: message ids can be reused

## Summary

`messages.id` is `INTEGER PRIMARY KEY`. When the newest message in the table is deleted, SQLite gives its id to the next one. Read marks only go up, so the new message can look read (review R10). User ids had the same problem (fixed with `last_user_id`).

## Requirements

- Message ids are never reused (a counter, or AUTOINCREMENT through a migration).

## Acceptance Criteria

- A test: read, delete the newest, post a mention; it is unread.
