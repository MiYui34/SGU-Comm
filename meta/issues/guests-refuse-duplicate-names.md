# Guests: refuse a name another web user has

## Summary

Part of [User accounts and registration behind the proxy](user-accounts-and-registration.md). Murmur kicks an older session when a new one has the same name and the same IP or certificate. All web users share the server's IP, so one guest can kick another by using the same name. Measured on 2026-10-14: different client certificates do not help, the IP alone is enough.

## Requirements

- The server reads the client's Authenticate message (the first frames it sends).
- If another web guest has that name, the server answers with a Mumble Reject (UsernameInUse) and closes, without connecting to Murmur.
- Desktop users have other IPs: Murmur already rejects the newcomer there.

## Acceptance Criteria

- A second web guest with a name in use gets "name in use", and the first one stays connected.
