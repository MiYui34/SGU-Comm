# mumbled: text channel mirrors and user moves

## Requirements

- SyncTextChannels(want): keep a "Text channels" parent (id saved in
  mumble_meta) and one "#name" child per entry, in order; create, rename,
  reorder, delete; returns the Mumble ids. Parent ACL denies Speak,
  Write, MakeChannel, MakeTempChannel, LinkChannel (here and below).
- OnUserMoved hook (join, move, leave), delivered in order outside the
  server lock.
- SendServerText(session, html): a text to one user, from the server.

## Acceptance Criteria

- Tests for sync (create, rename, reorder, delete, missing parent),
  permissions in mirrors, move events and server texts.
