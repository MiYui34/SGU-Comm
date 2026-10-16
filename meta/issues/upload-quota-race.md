# Uploads: parallel uploads get past the daily quota

## Summary

`files.go` reads the bytes used and records the upload in two steps. Parallel uploads all see the same allowance (review R6).

## Requirements

- Reserve the size per author before the save (a lock per author is enough), for browser uploads and images from Mumble clients. Free it again on failure.

## Acceptance Criteria

- A test: two parallel uploads over the quota; one is refused.
