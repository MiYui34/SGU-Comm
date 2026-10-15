# Search messages

## Summary

Search the messages of text channels.

## Requirements

- Search request (text, optional channel and author); SQLite full-text
  index; results link to the message.
- Only channels the user may read.

## Acceptance Criteria

- Store, sidecar and web tests.

## Result

- Server: FTS5 index (triggers; built once for older databases, in one
  transaction), quoted prefix terms (one letter: exact), control
  characters dropped, 200 characters at most, only readable chats, 25
  newest results. Web: search field in the chat header, results panel
  (count announced, focus handling), a click opens the text channel and
  pages back until the message is loaded (10 pages at most), then
  scrolls there; results follow edits and deletes.
- Tested: store, sidecar and web tests; browser: a result 60 messages
  back is loaded, scrolled to and flashed.
- Limit: voice chat results only show in the panel.
