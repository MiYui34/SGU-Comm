# Upload quota and captions

## Summary

Follow-ups from the upload review.

## Requirements

- A byte quota per author (per day) or a global disk limit. Today only the per-file size and the request rate limit uploads.
- Let the user add a caption: today an upload is sent at once with empty text. Discord shows a preview with a text field.
- Send the upload token in a header instead of the query string (proxy logs).

## Acceptance Criteria

- Uploads above the quota are refused with a clear error.
- An upload can carry text.
