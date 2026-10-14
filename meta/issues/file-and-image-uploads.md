# File and image uploads

## Summary

Upload files and images to text channels, stored by the sidecar. Part of [Sidecar: Discord-style server features](sidecar-discord-style-server-features.md).

## Requirements

- Upload with size limit, served back by the sidecar.
- Images show inline in the chat.

## Acceptance Criteria

- An uploaded image shows inline for all users in the channel.

## Notes

- The chat sanitizer allows only data: images today. Allow images from our own server origin.
