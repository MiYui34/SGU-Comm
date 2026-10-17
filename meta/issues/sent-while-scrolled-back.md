# Sending while scrolled back: notice, no jump

## Summary

Bug report: sending a message while scrolled back does not jump to the
bottom. Jumping may not be wanted (the reader loses the place). Instead
the "Jump to present" bar says "Message sent" with "Jump to your
message" (replaces the jump added in jump-to-present).

## Acceptance Criteria

- Sending while scrolled back keeps the place and shows the notice; the
  button goes to the bottom; at the bottom the notice is gone.
