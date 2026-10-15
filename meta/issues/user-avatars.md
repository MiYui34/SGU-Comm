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

## Result

- Done: settings "Profile picture" (square crop, 128 px JPEG), pictures
  in every avatar via a context lookup by name, fetched by hash on demand.
- Tested in the browser: a guest and a web account set pictures, another
  user sees them, removing works, a web account keeps its picture after a
  reconnect. Not checked visually in the official client (it shows
  avatars only in tooltips); server tests cover sending textures.
- Known limits: offline guests in chat history show the picture of an
  online guest with the same name; large-pixel images are decoded at full
  size by each viewer; our own long picture is downloaded back once.
