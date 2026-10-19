# Collapse very long messages

## Summary

The server limits a message to 2000 characters, but 2000 newlines still
make a message many screens tall. Messages taller than a limit (e.g. 20
lines or ~400 px) show collapsed with a fade and "Show more"; "Show
less" folds them again. Code blocks count too. (Discord does not collapse
messages; it limits length and turns long pasted text into a file. A
collapse is friendlier to read past spam.)

## Acceptance Criteria

- Pure functions for when to collapse (line count of the source text) with
  tests; the measured height decides only as a fallback (images).
- Expanded state per message kept while the chat is open.
- Keyboard and screen readers: the button says "Show more of the message".
