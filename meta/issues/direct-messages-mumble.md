# Direct messages: Mumble clients

## Requirements

- A private message from a desktop Mumble client to a registered user (both registered) is stored and reaches the web user once (not twice).
- A stored message from the web reaches the recipient's desktop Mumble client as a private message.
- A desktop client that joins gets the unread messages sent while it was away.

## Acceptance Criteria

- mumbled and sidecar tests for both directions and the catch-up.

## Notes

- A message that reaches one of the recipient's Mumble clients counts as read (also for the web badge), unless older ones are unread: the next catch-up sends those.
- If storing fails, a web recipient does not get a private message from a Mumble client (rare; logged).
