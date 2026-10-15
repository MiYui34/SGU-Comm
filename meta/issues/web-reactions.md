# Reactions on messages

## Summary

Emoji reactions under a message with a count, as in Discord.

## Requirements

- Registered users and guests in the channel may add and remove their own
  reaction (one per emoji per user). Stored per message; deleted with it.
- Everyone in the channel sees changes live; history comes with the counts
  and who reacted.
- A small picker with common emoji; clicking an existing reaction toggles
  ours.

## Acceptance Criteria

- Store and sidecar tests: add, remove, no duplicates, no reactions where the
  user may not read, live events.
- Reactions show and toggle in the browser.

## Notes

- Guests: names are free, so their reactions count by name like messages.
