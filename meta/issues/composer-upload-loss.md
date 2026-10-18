# Paste or drop during an upload loses the file

## Summary

Astra #13: only the attach button checks 'uploading'. Paste and drop set
a pending file; upload() returns when busy, but submit clears file and
caption anyway. A failed upload loses them too. Clear only when the
upload was accepted; keep them on failure.

## Acceptance Criteria

- Tests for the rule; manual check with a slow upload.
