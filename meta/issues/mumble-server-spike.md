# Mumble server: spike with Grumble and a minimal Go server

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md). Find out how
much work the server is, and if Grumble can help.

## Requirements

- Check Grumble: last activity, license, protocol version (1.5 protobuf UDP?),
  code quality, what is usable (crypt, ACLs, routing, storage).
- A minimal Mumble server in the Go module (behind a flag, Murmur stays the
  default): TLS, Version/Authenticate/ServerSync, one channel, UserState and
  UserRemove, TCP and UDP voice routing, ping.

## Acceptance Criteria

- The official Mumble client connects to the minimal server, and two clients
  hear each other (over UDP).
- A short written result: use Grumble, copy parts of it, or write our own,
  with an estimate for the full server.

## Notes

- Tests can use the existing bridge and protocol tests, which run against a
  real Murmur now.
