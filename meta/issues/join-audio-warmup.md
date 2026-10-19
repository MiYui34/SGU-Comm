# Audio is rough for seconds after opening the app

## Summary

Report (Linux AppImage, also milder on the web): after opening the app and
joining voice, audio is jerky with high late audio for ~5 s; it starts
smooth only after the chat history has filled. Switching channels later is
fine. Likely head-of-line blocking: voice, history pages, previews and all
users' avatars (Mumble texture blobs, up to ~120 KB each) share one
WebSocket (TCP), so voice waits behind bulk data; plus main-thread work
(rendering) competing with audio decoding, worse in WebKitGTK.

## Acceptance Criteria

- Measure first: bytes and timing per message type on the socket in the
  first 10 s after start/join, with many users with avatars and a
  throttled link; report.
- Then fix what the numbers show, e.g. avatars over HTTP (not the voice
  socket), voice before history/previews, voice priority in the server's
  send queue, less main-thread work at start.
- Before/after numbers (late audio in the first 10 s).
