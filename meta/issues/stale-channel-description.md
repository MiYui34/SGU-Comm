# Web: a changed long channel description can stay old

## Summary

Long descriptions come as a hash. The client remembers fetches by channel id, not by (channel, hash), so a new hash is not fetched (review, lower priority).

## Requirements

- Fetch again when the hash changes, as for avatars.

## Acceptance Criteria

- A protocol state test: a new hash leads to a new fetch.
