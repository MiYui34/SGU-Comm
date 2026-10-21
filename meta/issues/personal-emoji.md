# Personal emoji

## Summary

Each registered user curates their own emoji (Telegram/Signal style), so nobody has to ask admins to add one. Everyone sees a personal emoji in messages and reactions; only using it needs it in your own list. Emoji spread with "Add to my emoji" on an emoji in a message.

## Requirements

- Registered users (not guests) add, rename and delete their own emoji in their user settings, with the same image checks and auto-resize as server emoji. Limit: 50 per user.
- Images are stored once per unique image (content hash), shared between users who have it.
- Messages and reactions reference a personal emoji unambiguously (by image id, like Discord's `<:name:id>`). The user types `:name:`; the client inserts the reference. Server emoji keep priority for their names and stay name-based.
- Everyone renders a referenced personal emoji, whether or not they have it.
- "Add to my emoji" on a personal or server emoji in a message or reaction (name taken from the emoji, changeable on a clash).
- Picker and :shortcode: suggestions have a "My emoji" section.
- Mumble clients see `:name:` as plain text (mirror text and history).
- Moderators remove an emoji image everywhere (old messages show `:name:` text) and block that image from being added again.

## Acceptance Criteria

- Server: storage, limits, permission checks, references in messages/reactions, mirror text, moderation; tests.
- Client: settings management, picker section, suggestions, rendering, "Add to my emoji"; tests for the pure logic.
- Headless browser check of add, use, collect from a message, and moderator removal.

## Notes

- Later: shareable packs (one link adds a set).
- Defaults chosen: registered only, 50 per user, no approval step.
