# Rewrite the server in Rust or Go

## Summary

Port the WebSocket-to-Mumble bridge from TypeScript to Rust or Go before the sidecar features are added. Part of [Sidecar: Discord-style server features](sidecar-discord-style-server-features.md).

## Requirements

- Same behaviour as the TS proxy: origin check, connection limit, max payload, backpressure, error handling (see packages/proxy tests).
- Same environment variables.
- Decide between Rust and Go first, and record the decision in DEVLOG.md.

## Acceptance Criteria

- The web client works unchanged against the new server.
- The proxy test cases are ported and pass.
- The TS proxy is removed, or kept only for the dev scripts.

## Notes

- The user prefers Rust or Go. Rust fits a future Tauri app; both have good WebRTC libraries (webrtc-rs, pion).
- Keep a WebRTC voice bridge in mind as a later option, if voice over TCP is too laggy on bad networks.
