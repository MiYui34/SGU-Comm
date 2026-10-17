# Keyboard: quick switcher and channel keys

## Summary

Ctrl/Cmd+K opens a quick switcher (fuzzy search for channels and
users/DMs). Alt+Up/Down goes to the channel above/below, Alt+Shift+Up/
Down to the next unread one. Esc in the message box marks the channel
read. A shortcut list (Ctrl+/).

## Acceptance Criteria

- Unit tests for matching and channel order; manual check.

## Notes

- Not done: Esc to mark a channel read. Opening a channel already marks
  it read, so it would do nothing.
