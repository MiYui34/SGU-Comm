# Images in Mumble messages

## Summary

Mumble clients send pictures in chat as HTML with data: images (Murmur
allows 128 KiB for such messages). The built-in server drops messages over
5000 characters and stores only their text, so pictures from Mumble users
(and bots, for example a music bot's covers) never reach the web app.

## Requirements

- Messages with images may have up to 128 KiB; plain text stays at 5000.
- Images (PNG, JPEG, GIF, WebP by their bytes) in a message to a stored
  chat (voice chat or text channel mirror) are stored as attachments; the
  text goes along as usual.
- Live Mumble clients get the message unchanged.

## Acceptance Criteria

- mumbled, sidecar and files tests; a Mumble message with an image shows
  as an image attachment in the web app.
