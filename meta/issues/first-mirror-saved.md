# First text channel's Mumble copy not saved

## Summary

Astra #19: a new mirror channel at position 0 with an empty topic looks
unchanged, so it is not saved nor announced until a restart (then made
again with a new id). Mark new mirrors as changed.

## Acceptance Criteria

- Test: creating the first text channel saves and announces its mirror.
