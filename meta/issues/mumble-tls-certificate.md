# Mumble port with the web certificate

## Summary

Mumble clients get a "certificate not trusted" prompt: the built-in
server uses a self-signed certificate.

## Requirements

- Optional MUMBLE_CERT_FILE / MUMBLE_KEY_FILE (for example Caddy's Let's
  Encrypt files); reloaded when they change; self-signed otherwise.
- deploy files show how to share Caddy's certificate.

## Acceptance Criteria

- Tests for loading and reloading; official client connects without the
  prompt.
