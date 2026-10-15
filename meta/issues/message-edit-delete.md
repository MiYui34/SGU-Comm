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
