# One "Go live" button

## Summary

The voice bar has separate camera and screen buttons, and OBS is hidden at
the bottom of Settings. One "Go live" button (screen icon, highlighted
while any of your streams runs) opens a small menu above it:

- Share your screen / Stop sharing (not on phones)
- Turn on camera / Turn off camera
- Stream from OBS… : a dialog with the steps, the WHIP URL and the stream
  key (moved here from Settings; only for accounts with a password and
  where the server has WHIP)

## Acceptance Criteria

- Pure functions for the menu entries (by state, phone, account, server
  features), with tests.
- Keyboard: arrow keys, Enter, Escape; focus returns to the button.
- Manual check: start and stop each from the menu; OBS dialog makes a key.
