# User accounts and registration behind the proxy

## Summary

Identity for web users. TLS ends at the proxy, so browsers cannot present their own Mumble client certificate, and all web users are unregistered today. Part of [Sidecar: Discord-style server features](sidecar-discord-style-server-features.md).

## Requirements

- Accounts in the sidecar.
- Per-user Mumble client certificates stored by the server, so web users can register on Murmur.
- Handle Murmur's same-name kick: an unregistered user with the same name from the same IP kicks the older session, and all web users share the proxy IP.

## Acceptance Criteria

- A web user can log in, register on the Mumble server, and keep their registration across sessions.
