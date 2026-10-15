# Mumble port with the web certificate

## Summary

Mumble clients get a "certificate not trusted" prompt: the built-in
server uses a self-signed certificate.

## Requirements

- Optional MUMBLE_CERT_FILE / MUMBLE_KEY_FILE (for example Caddy's Let's
  Encrypt files); reloaded when they change; self-signed otherwise.
- deploy files show how to share Caddy's certificate.
- Desktop app: certificates valid for the host name (web PKI) are trusted
  without a pin, so renewals do not ask again; self-signed ones are pinned
  as before.

## Acceptance Criteria

- Tests for loading and reloading; official client connects without the
  prompt.

## Status

- Done in code: MUMBLE_CERT_FILE / MUMBLE_KEY_FILE (reloaded when they
  change, self-signed until they exist); the desktop app trusts web PKI
  certificates without a pin (unit tests, and a network test against a
  Let's Encrypt certificate).
- Open: use it on a deployed server (copy the proxy's certificate where
  the server can read it), then check the official client gets no prompt.

## Result

- In use on a deployed server: the official client (1.5) connects without
  the certificate prompt; openssl verifies the chain; the desktop app's
  network test sees it as trusted.
