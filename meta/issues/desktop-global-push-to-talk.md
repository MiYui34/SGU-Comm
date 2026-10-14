# Desktop: global push-to-talk

## Summary

Part of [Desktop app (Tauri)](desktop-app-tauri.md). Browsers only see keys while the tab has focus. The desktop app registers the push-to-talk key globally.

## Requirements

- Global shortcut (tauri-plugin-global-shortcut) with press and release events, wired to capture.setPttDown.

## Acceptance Criteria

- With the app in the background, holding the key sends voice.
