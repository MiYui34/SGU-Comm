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
