# Remember self-mute and deafen across a page refresh

## Summary

Self-mute and deafen reset on a page refresh. Keep them, as Mumble keeps them across restarts.

## Requirements

- Self-mute and deafen are saved per server in localStorage and restored when the client connects (before any voice join).
- A reconnect keeps using the in-memory state as now.

## Acceptance Criteria

- Unit tests for load/save; web tests and svelte-check pass; a headless check that a muted user is still muted after a refresh.
