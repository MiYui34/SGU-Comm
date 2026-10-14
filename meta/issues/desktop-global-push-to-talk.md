# Desktop: global push-to-talk

## Summary

Part of [Desktop app (Tauri)](desktop-app-tauri.md). Browsers only see keys while the tab has focus. The desktop app registers the push-to-talk key globally.

## Requirements

- Global shortcut (tauri-plugin-global-shortcut) with press and release events, wired to capture.setPttDown.

## Acceptance Criteria

- With the app in the background, holding the key sends voice.

## Status (2026-10-14)

- Implemented: tauri-plugin-global-shortcut, registered only in push-to-talk mode, released on unregister. Unit tests for the registration logic; the app builds.
- Not verified: the development sandbox cannot run the app window. Needs a manual test: app in the background, hold the key, voice goes out.
