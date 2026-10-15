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

## Result

- Server: read marks per registered name and channel; ready carries them
  and a summary (newest message, unread mentions, voice chats with their
  voice channel). Web: unread and mention badges from the summary plus
  live messages; marks go back at most every 2 seconds and again after a
  reconnect; no sound for mentions from before.
- Tested: store, sidecar and web tests; browser: mention while offline,
  badge after login, gone after reading and a reload.
- Limit: a channel open in a hidden tab counts as read.
