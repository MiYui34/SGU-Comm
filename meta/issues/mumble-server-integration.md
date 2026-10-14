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
