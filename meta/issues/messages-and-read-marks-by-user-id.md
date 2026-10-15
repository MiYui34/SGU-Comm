# Messages and read marks by user id

## Summary

Message ownership (edit, delete) and read marks use the user id; existing
data is migrated from names (registered names are unique).

## Acceptance Criteria

- Tests: a renamed user keeps edit rights and read marks; the migration maps
  names to ids.
