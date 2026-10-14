# Desktop app (Tauri)

## Summary

A desktop shell for the same UI, with a native Mumble connection.

## Requirements

- Tauri shell around the web app.
- Native TCP/TLS and UDP to Murmur, so no proxy is needed.
- Global push-to-talk hotkey (browsers only allow PTT while the tab has focus).

## Acceptance Criteria

- The desktop app connects straight to a normal Mumble server, with voice and chat.

## Notes

- The protocol package runs in the browser already; only the transport changes.
