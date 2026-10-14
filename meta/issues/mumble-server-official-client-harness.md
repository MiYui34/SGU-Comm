# Mumble server: test harness with the official client

## Summary

Part of [Built-in Mumble server](built-in-mumble-server.md). The official
Mumble 1.5 client in podman (Xvfb), so every step is tested with it.

## Requirements

- A podman image with the Mumble 1.5 client and Xvfb, and a script that
  connects it to a server address with a name.
- A Go test (skipped without the env variable) that checks the official
  client joins and hears voice, through our test client.

## Acceptance Criteria

- The official client connects to `mumbled`, and its user appears in the
  user list of another client.
