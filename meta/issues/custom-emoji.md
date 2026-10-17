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
