# Server: ACL edits do not check permission again after the lookup

## Summary

`aclMsg` checks Write, unlocks for a database lookup, then writes. Permission from a dynamic group like `@in` can be gone by then (review, lower priority).

## Requirements

- Check Write again after the lock is taken again.

## Acceptance Criteria

- A test, or a short code change with a comment if a test is impractical.
