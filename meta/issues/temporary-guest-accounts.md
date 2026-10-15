# Temporary guest accounts

## Summary

"Join as guest" creates an account without a password (session cookie,
Mumble registration). Setting a password keeps it. Accounts without a
password that were not seen for a day are deleted.

## Requirements

- POST /api/guest {name}: the same name rules as accounts; a per-IP limit.
- /api/me says whether the account is temporary.
- Setting a password on a temporary account needs no old password and makes
  it a normal account.
- Cleanup (built-in server only): no password, not seen for 24 h, not
  online: delete the account, its sessions and its Mumble registration
  (with comment and avatar). Messages stay.
- The web client joins as a guest account where the server has them, else
  as a classic guest; the settings offer to set a password.

## Acceptance Criteria

- Auth and store tests: create, limits, upgrade, cleanup rules.
- In the browser: join as guest, reload (same account), set a password.
