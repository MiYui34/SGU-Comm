# Phone: the member drawer pushes the chat aside when it opens

## Summary

Opening the member list on a phone shifts the chat 280 px to the left and lets it slide back. The drawer's first button gets focus while the drawer is still off-screen to the right, and the browser scrolls the `overflow: hidden` layout to show it. The channel drawer (off-screen left) cannot scroll, so it is fine.

## Requirements

- Drawer focus does not scroll (`preventScroll`); the phone layout cannot be scrolled (`overflow: clip`).

## Acceptance Criteria

- Headless check at phone size: `.layout` scrollLeft stays 0 while the member drawer opens and closes.
