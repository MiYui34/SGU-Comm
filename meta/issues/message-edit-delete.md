# Edit and delete messages

## Summary

Users edit and delete their own messages in text channels and voice
channel chats; moderators and admins delete anyone's.

## Requirements

- Sidecar requests edit-message and delete-message; the server checks the
  author (verified names only for others' rights) or MayManage.
- Edited messages show "(edited)"; deleted ones go away for everyone.
- Web: hover actions and the message menu (Edit, Delete); Up arrow in an
  empty box edits my last message.
- Mumble clients cannot edit; mirrors get nothing for edits.

## Acceptance Criteria

- Store, sidecar and web tests; browser check with two users.

## Result

- Done. Server: verified authors by name, guests only messages from their
  current connection (forgotten on disconnect: Murmur reuses sessions),
  moderators and admins delete any; deleting removes the attachment file.
  Web: hover/focus actions (always shown on phones), inline edit, Up arrow,
  confirm with focus and Escape, "(edited)".
- Tested: store, sidecar and web tests; browser with a guest, a second
  user and SuperUser.
- Limits: edits and deletes do not reach Mumble clients; ownership goes by
  name (a renamed account loses its old messages).
