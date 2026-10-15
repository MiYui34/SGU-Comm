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

## Result

- Messages with images up to 128 KiB; images checked by their bytes and
  sent to Mumble clients in server-built tags; other HTML removed until
  nothing is left (a nested tag got through one pass before).
- Images in messages written directly to a stored chat are kept as
  attachments, for registered users only and within their daily upload
  quota (guests can use any name); tree messages keep only the text.
- Tested: mumbled, files and sidecar tests, the official client suite; a
  music bot's covers show as images in the web app.
