# Mumble server: spike with a minimal Go server

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md). Find out how
much work the server is.

## Requirements

- A minimal Mumble server in the Go module (behind a flag, Murmur stays the
  default): TLS, Version/Authenticate/ServerSync, one channel, UserState and
  UserRemove, TCP and UDP voice routing, ping.

## Acceptance Criteria

- The official Mumble client connects to the minimal server, and two clients
  hear each other (over UDP).
- A short written result with an estimate for the full server.

## Notes

- Tests can use the existing bridge and protocol tests, which run against a
  real Murmur now.
- Grumble was checked briefly and is not a base (user decision: peripheral).
  Findings: BSD, about 14k lines, 8 commits since 2023, announces 1.2.5,
  legacy UDP only (no 1.5 protobuf format). Its OCB2 output is identical to
  ours. It can be a reference for ACL semantics, nothing more.
