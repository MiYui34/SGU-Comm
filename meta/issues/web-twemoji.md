# Web: Twemoji images for emoji

## Summary

Emoji look different per system, and Safari in Lockdown Mode shows none. Show Twemoji as images (served by the Kallisti server), not as a font.

## Requirements

- Emoji in messages, reactions and the emoji picker are Twemoji images; the text stays selectable/copyable (alt text).
- Images come from the Kallisti server (CSP: self); the license attribution is in the third-party notices.

## Acceptance Criteria

- Unit tests for the replacement; checked in the browser.
