# Desktop: GPU use while the window is minimised

## Summary

GitHub #3: with animated pictures or emoji visible in the chat, the desktop app keeps using the GPU (about 6%) while minimised. backgroundThrottling is disabled (for voice), so the web view keeps painting.

## Requirements

- While the window is hidden or minimised, animated content in the chat does not paint (animated images, emoji, avatars, CSS animations), and comes back when the window shows again.
- Signals: document visibility plus Tauri window events (minimised, restored), since WebKitGTK may not report a minimised window as hidden.
- Voice, notifications and message handling keep running.

## Acceptance Criteria

- Unit tests for the hidden-state logic; web tests and svelte-check pass; headless check that hiding the page sets the state and hides animated content.
