# Linux desktop app: no audio

## Summary

Report: in the Linux AppImage, voice works neither in nor out. Likely:
WebKitGTK refuses getUserMedia unless the app enables media streams and
answers its permission request (we do neither); WebKitGTK versions often
lack WebCodecs AudioEncoder/AudioDecoder (our Opus path); the AppImage may
miss GStreamer plugins.

## Acceptance Criteria

- Linux: WebKitGTK media stream enabled; microphone/camera permission
  requests of our own page allowed (Rust, with_webview).
- Opus in WebAssembly as a fallback when WebCodecs audio is missing
  (encode and decode), with tests for the selection.
- CI: the AppImage's WebKitGTK version known; GStreamer media bundled if
  needed.
- Checked in a Linux container if possible; final check by a Linux user.
