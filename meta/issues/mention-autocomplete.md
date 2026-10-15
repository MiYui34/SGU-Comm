# Mention autocomplete

## Summary

Typing "@" in the message box suggests names to mention.

## Requirements

- After "@" (at the start or after a space), a list of matching names:
  online users and recent authors of the open channel, not me; names that
  start with the typed text first; at most 8.
- Up/Down choose, Enter or Tab insert "@Name ", Escape closes; a click
  inserts too. Enter sends only when the list is closed.
- Screen readers: the list is a listbox tied to the message box.

## Acceptance Criteria

- Unit tests for finding the query, suggesting and inserting; browser
  check.

## Result

- Done as described (combobox with listbox; Up/Down, Enter/Tab, Escape,
  click; not during input method composition). Tested in the browser with
  two users: list, picking with Enter and Tab, Escape, sending, reopening.
