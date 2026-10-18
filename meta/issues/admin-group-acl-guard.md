# Only SuperUser removes admins, also through Mumble's ACL editor

## Summary

set-admin lets only SuperUser remove admins, but any admin has Write in
root and can edit root's ACL with a Mumble client: a new "admin" group
without another admin removes that admin. The ACL message must keep the
admins of root's "admin" group unless SuperUser sends it.

## Acceptance Criteria

- Server test: an admin's ACL edit of root that drops another admin from
  the "admin" group keeps that admin; SuperUser's edit removes them.
- Adding admins through the ACL editor still works (as set-admin allows).
