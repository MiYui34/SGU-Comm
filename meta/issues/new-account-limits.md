# Limits for new accounts

## Summary

Optional: accounts younger than N minutes (and guests) may not post
links, uploads or mentions of @everyone/@here, and get a lower message
rate. Admins choose N in the server settings (off by default).

## Acceptance Criteria

- Server tests: the limits apply only to new accounts and guests; staff
  exempt; a clear error message.
- Web: the setting; the error shows in the composer.
