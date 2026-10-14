# Desktop: trust on first use for server certificates

## Summary

Part of [Desktop app (Tauri)](desktop-app-tauri.md). The local bridge accepts any server certificate (signatures are checked). A network attacker could read the session, including the server password, without a warning. Required before a public release.

## Requirements

- On the first connection to a server, show the certificate fingerprint and ask the user.
- Store the accepted fingerprint per server; warn and refuse when it changes.

## Acceptance Criteria

- A changed server certificate is refused with a clear warning, until the user accepts the new one.

## Result

- Pins per host:port in the app data folder (written by Rust only). Unknown: trust question with the fingerprint; changed: warning, refused until accepted. Tested with Rust tests and in Chrome through the bridge example (pin file written directly; the Tauri window could not run here).
- Found on the way: closing with unread client data made browsers see code 1006 without the reason; the bridge now waits for the close reply.
