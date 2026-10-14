# Mumble server: integration into the Kallisti server

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md).

## Requirements

- The Kallisti server runs the Mumble server in-process. Web clients join
  without a TCP/TLS hop; Kallisti accounts are Mumble registrations.
- The sniffer and per-account client certificates are no longer needed.
- Text channels keep working.
- Docker and compose files without Murmur.

## Acceptance Criteria

- The full stack (web client, desktop app, official client) works with one
  container and no Murmur.
- One name space for web accounts and Mumble registrations: a web account
  must be a mumble_users row (or mumble_users gets an account id), so a
  native guest cannot self-register a web account's name.
- identify() reads the DB directly; the in-process path needs a lookup hook
  for Kallisti accounts.
- Pass the real client IP (internal/clientip) into the session: the guest
  ghost rule uses the IP, and behind the bridge all web users share one.
- Look-alike names (Cyrillic, fullwidth) can pass as registered names, as
  in Murmur. NFKC in names.Key would fold fullwidth forms; that needs a
  migration of stored name keys.
