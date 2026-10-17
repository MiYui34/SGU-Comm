# Clicking a reply quote jumps to the original

## Summary

A user report: clicking the quote above a reply does nothing. When the
original message is not loaded (older than the loaded pages), `jumpTo`
returns at once. Search results already page back until the message is
loaded; quotes should do the same.

## Requirements

- A click on a quote loads older pages (at most as many as for search)
  until the original is loaded, then scrolls to it and highlights it.
- A quote of a deleted message does not load pages.
- When the original is not found, the search stops (no later surprise
  jump).

## Acceptance Criteria

- Unit tests for the paging decision.
- Manual check: a quote of a message outside the loaded pages scrolls there.
