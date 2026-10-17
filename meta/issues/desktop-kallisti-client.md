# Desktop app as a Kallisti client

## Summary

Bug report: password login does not work in the desktop app. Cause: the
desktop app is only a Mumble client. When the address is a Kallisti
server, the desktop app should work like the web app (accounts, guests,
text channels, all features); plain Mumble servers stay as today.

Decisions (2026-10-18): voice over the Kallisti WebSocket first (native
UDP for Kallisti servers later); the login token in the app's storage.

## Sub-issues

- desktop-access-tokens: the server accepts access tokens and the Tauri
  origins.
- desktop-kallisti-mode: the desktop app finds Kallisti servers and runs
  the web client against them.
- mumble-password-for-accounts: Kallisti accounts log in with their
  password from Mumble clients.
