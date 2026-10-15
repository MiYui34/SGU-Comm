# Channel menu on phones

## Summary

Phones have no right-click: channels cannot be edited, moved or deleted.

## Requirements

- Long press on a channel opens its menu (and a user's menu).

## Acceptance Criteria

- Unit test for the long-press timing; browser check with touch events.

## Result

- A long press (500 ms, without moving) sends a contextmenu event, so the
  channel menu opens; Android's own context menu is not doubled. User rows
  already open their menu with a tap. Tested with fake timers and with
  synthetic touch events in the browser; not on a real phone.
