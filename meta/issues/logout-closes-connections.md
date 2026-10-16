# Logout everywhere and password change do not close open connections

## Summary

The bridge checks the cookie once. "Log out everywhere" and a password change delete the sessions, but open WebSocket connections stay and can still post (review R4).

## Requirements

- Keep the open connections per session and account. Close them on logout, logout everywhere (all of the account's) and password change (all but the current session).

## Acceptance Criteria

- A test: after logout everywhere, an open connection is closed.
