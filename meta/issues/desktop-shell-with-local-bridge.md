# Desktop: Tauri shell with a local bridge

## Summary

Part of [Desktop app (Tauri)](desktop-app-tauri.md). A Tauri app around the web client. A small bridge in Rust (WebSocket on 127.0.0.1 to TLS) lets the unchanged web client talk to any Mumble server directly, without the Kallisti server.

## Requirements

- Tauri v2 app in `apps/desktop`, loading the built web app.
- Local bridge in Rust: WebSocket on 127.0.0.1 (random port, random secret) to TLS to the chosen Mumble server.
- Login screen in the desktop app: server address field.
- Text channels, uploads and accounts are simply unavailable against a plain Mumble server (as in the web client).

## Acceptance Criteria

- An automated test connects through the local bridge to Murmur and syncs.
- The app builds on macOS.

## Result

- Automated tests: TLS echo and a real Murmur through the bridge. Release build produces Kallisti.app. The window itself could not be opened in the development sandbox.
