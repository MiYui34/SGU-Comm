# Fetch long channel descriptions with RequestBlob

## Summary

Murmur sends `description_hash` instead of `description` when a description is 128 bytes or longer. Such channels show no description today.

## Requirements

- Store descriptionHash in the channel state.
- Request missing descriptions with RequestBlob when needed (for example on hover).

## Acceptance Criteria

- A channel with a long description shows it in the UI.

## Notes

- User comments (comment_hash) work the same way.
