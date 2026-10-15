# User avatars

## Summary

Users can set a profile picture. Built on Mumble's user texture, so
normal Mumble clients show the same pictures and the built-in server
stores them for registered users (web accounts too).

## Requirements

- Settings: choose a picture (scaled down in the browser) or remove it.
- Avatars show the picture wherever they show the initial now (sidebar,
  member list, user menu, chat, user panel); the initial stays as the
  fallback.
- Pictures of others load on demand (texture hash, RequestBlob).
- Only images are shown (PNG, JPEG, GIF, WebP), within the server's size
  limit.

## Acceptance Criteria

- A web account sets a picture; another web user and the official Mumble
  client see it; it is still there after a reconnect.
- Removing it brings the initial back for everyone.
- Tests in protocol and web.
