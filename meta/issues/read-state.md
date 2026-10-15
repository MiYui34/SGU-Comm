# Remember what was read

## Summary

Unread marks and mention badges live only in memory: a reload or a second
device loses them. Store the last read message per channel for web
accounts.

## Requirements

- Server keeps last read per account and channel; a mark-read request;
  sent with ready.
- Unread and mention counts work for channels not opened in this session
  (the server sends unread and mention counts per channel).
- Guests keep the in-memory behaviour.

## Acceptance Criteria

- Store, sidecar and web tests; reload keeps the marks.
