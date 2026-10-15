# Bans and users in the web app

## Summary

Moderators and admins see and manage bans (and registered users) only
through the official Mumble client. Add a small view in the web app.

## Requirements

- Ban list: see, remove (admins: Write in root; moderators read only).
- Registered users: see, rename, unregister (rules as on the server).
- Uses the Mumble protocol messages (BanList, UserList).

## Acceptance Criteria

- Protocol and web tests; browser check as admin and moderator.

## Result

- "Server settings" (gear next to the server name, for Ban or Register in
  root): ban list (admins remove entries; moderators read) and registered
  users (rename, unregister with confirmation). Protocol: BanList and
  UserList in the state, actions, readable ban addresses.
- Found and fixed on the way: renaming a web account's registration locked
  the account out; the server refuses it now and says why.
- Tested: protocol, mumbled and web tests; browser as SuperUser (rename
  refused for an account, rename and unregister of a plain registration).
  Removing a ban is not tested in the browser (no ban in the test data).
