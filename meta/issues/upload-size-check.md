# Check the upload size before uploading

## Summary

The client uploads files larger than the server's limit and learns only
from the refusal. The server tells clients its limit (ready event); the
composer refuses larger files at once: "The file is too large (at most
10 MB)."

## Acceptance Criteria

- Server test: the ready event carries the limit.
- Web tests for the size check and the message.
