# Connections drop in background tabs

## Summary

Browsers slow JavaScript timers in background tabs (Chrome: once a minute
after about 5 minutes). The client's 15 s ping then comes too late, and the
Mumble server drops the client after 30 s without data: "Lost the
connection to the server" after switching tabs.

## Requirements

- Sub-issues:
  - [ ] [Bridge keeps connections alive](bridge-keepalive.md)
  - [ ] [Reconnect automatically](auto-reconnect.md)

## Acceptance Criteria

- A web client whose page sends nothing for minutes stays connected.
- A dropped connection comes back by itself, in the same place.
