# Custom server emoji

## Summary

Moderators and admins add server emoji (a name and a small image) and
delete them, in Server settings. Everyone uses them as :name: in messages
and as reactions; the picker has a section for them, and :shortcode:
suggestions find them.

## Acceptance Criteria

- Server: storage, permission checks, size and name limits; tests.
- Client: rendering in messages and reactions, picker, management tab.
- Mumble clients see :name: as text.

## Notes

- Images are blobs in the `emoji` table (AUTOINCREMENT ids, so
  `/api/emoji/{id}` can be cached for ever). Not in UPLOAD_DIR: the
  cleanup of unused uploads would delete them.
- Adding: `emoji-token` (name checked, admins and moderators) over the
  WebSocket, then `POST /api/emoji` with the image. Deleting: `delete-emoji`.
- Reactions of a deleted emoji stay as `:name:` text and can be taken back.
- Open: an emoji token stays valid for 5 minutes after its holder loses
  the moderator role; a reused name shows the new image in old messages.
