# Custom emoji in the custom status

## Summary

Bug: a server emoji (:name:) in the custom status is refused ("not an
emoji"); the status check knows only Unicode emoji. Accept existing
server emoji, as reactions do.

## Acceptance Criteria

- Server test: an existing :name: is accepted, an unknown one refused.
