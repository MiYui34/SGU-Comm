# Make admins in the web app

## Summary

Admins are the "admin" group in root; the web app can only make
moderators. Add "Make admin" to the user menu (admins may), and
"Remove admin" (only SuperUser may, so admins cannot remove each other).
Server: SetAdmin next to SetModerator.

## Acceptance Criteria

- [x] Server tests: admins make admins; non-admins cannot; only SuperUser
  removes admins; SuperUser itself cannot be changed.
- [x] Manual check in the user menu.
