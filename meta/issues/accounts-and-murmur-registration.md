# Accounts and Murmur registration

## Summary

Part of [User accounts and registration behind the proxy](user-accounts-and-registration.md). Web users can create an account. The server keeps a client certificate per account and uses it towards Murmur, so the user can register on Murmur and keep the name (and the verified mark).

## Requirements

- Accounts in SQLite: name, password hash (argon2id), certificate and key.
- HTTP API: create account, log in, log out, who am I. Session in an HttpOnly cookie. Rate limit for logins.
- The bridge dials Murmur with the account's certificate.
- The client: log in or create an account on the login screen, guests still possible; after the sync, an account that is not registered on Murmur registers itself (UserState with user_id 0, measured to work with the default ACL).

## Acceptance Criteria

- A user can create an account, log in, and appears registered (verified) on Murmur.
- After logging in again (new session), the user is still registered and keeps the name.

## Later (from the security review)

- "Log out everywhere"; a password change should end other sessions.
- With ALLOWED_ORIGINS=*, the API reflects any origin with credentials (development only); refuse credentials unless a dev flag is set.
- Warn at startup when the server seems to run behind a proxy without TRUST_FORWARDED_FOR (cookies not Secure, one shared rate limit).
