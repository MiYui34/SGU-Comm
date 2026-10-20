# Review 3: server races and authorization

## Summary

Some server mutations authorize against a snapshot taken before a wait, and a missing channel can look public.

## Requirements

- M1: A password change checks the current password hash and session at the time of the update (atomic), not a snapshot taken before the body was read.
- M2: A missing mirror channel is not treated as public. Delivery of posts and preview rebroadcasts fails closed when the channel is gone.
- M7: Category management requests authorize inside the serialized mutation, against the channel's current category.
- M13: A post into a thread checks that the thread still exists inside the insert transaction; a deleted thread refuses the post.

## Acceptance Criteria

- Regression tests for each item (deterministic interleavings where needed).
- Go race tests pass.
