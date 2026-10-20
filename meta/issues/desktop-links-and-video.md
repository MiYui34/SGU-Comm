# Desktop app: links do nothing, videos don't play (Linux)

## Summary

Report (Linux AppImage v0.9.2): an MP4 shows as a file card, and clicking
its name does nothing.
1. Links: chat links open with target="_blank"; the Tauri web view has no
   tabs and ignores new-window requests, so file links, message links and
   link previews likely do nothing in the desktop app (all platforms?).
   Open them in the system browser (tauri-plugin-opener), keep in-app
   links (channel links, message links to this server) in the app.
2. Video on Linux: WebKitGTK plays media through GStreamer; the AppImage
   bundles plugins-base/good only, so H.264/AAC (most MP4) cannot play and
   the client falls back to the file card. Bundle gst-libav and the needed
   plugins-bad parts (h264parse etc.).

## Acceptance Criteria

- Links: vitest for the link decision (external vs in-app); in podman, a
  click on an external link and a file link calls the opener (a fake
  xdg-open logs it).
- Video: in podman, the real AppImage plays an H.264/AAC MP4 (canplay,
  currentTime advances); the AppImage size change is noted.
