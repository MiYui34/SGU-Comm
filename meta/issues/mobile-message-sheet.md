# Phones: message actions on long press

## Summary

On touch screens the message buttons (react, reply, edit, delete) always
show and crowd the chat. Discord hides them; a long press opens a sheet.

## Requirements

- Touch screens: no buttons on the messages.
- A long press on a message opens a bottom sheet: quick reactions and "+"
  (the emoji picker), then Reply, Copy text, Pin, Edit, Delete, each only
  when allowed.
- Copy text: phones only (the written text, not the HTML).
- Closes on a tap outside, a drag down, or Escape; the message is marked
  while it is open.
- Desktop stays as it is.

## Acceptance Criteria

- Manual check on a phone (or touch emulation): long press opens the
  sheet; every action works; scrolling does not open it.

## Notes

- Uses lib/longPress. No swipe to reply (conflicts with the browser's back
  gesture).
