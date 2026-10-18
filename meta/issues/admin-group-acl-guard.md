# Only SuperUser removes admins, also through Mumble's ACL editor

## Summary

set-admin lets only SuperUser remove admins, but any admin has Write in
root and can edit root's ACL with a Mumble client: a new "admin" group
without another admin removes that admin (so does dropping the entry
that gives "admin" Write, or a later deny). So only SuperUser changes
root's ACL through a Mumble client; admins can still read it, and make
admins and moderators in the web app.

## Acceptance Criteria

- Server test: an admin's ACL edit of root is denied and changes nothing;
  an admin can still query it; SuperUser's edit works.
- ACL edits of other channels by admins still work.
