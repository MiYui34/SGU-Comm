# Emoji picker

## Summary

Reactions offer only 10 emoji (COMMON_EMOJI); the server accepts any.
A full picker, as in Discord, for reactions and the message box.

## Requirements

- All emoji of the Twemoji version we ship, with names and keywords
  (emojibase-data, same Unicode version), loaded only when the picker opens.
- Search field; "Recently used" first (stored in the browser); categories
  with a row of icons to jump to them; a skin tone choice.
- Twemoji images in the grid (text when the images are missing).
- Opens from "+" in the reaction row and from a new emoji button in the
  message box (inserts at the cursor).
- The quick reaction row: recent emoji first, then the defaults.
- Keyboard: arrow keys in the grid, Enter picks, Escape closes.

## Acceptance Criteria

- Unit tests for the data (groups, search, skin tones, recent list).
- Manual check: react with an emoji outside the defaults; insert one in a
  message; search finds "thumbs up" by "thumb" and "+1".

## Notes

- A popup on desktop; on phones it fills a bottom sheet (see
  mobile-message-sheet).
