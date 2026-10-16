# Web: multiline messages and channel links

## Summary

Messages can only be one line, and "#random" in a message is plain text.

## Requirements

- Shift+Enter makes a new line; Enter sends.
- The message field grows with its text (up to a limit) and wraps long lines.
- Line breaks show in the chat (and reach Mumble clients as line breaks).
- "#name" of an existing text channel links to that channel.

## Acceptance Criteria

- Unit tests for the rendering (line breaks, channel links); checked in the browser.
