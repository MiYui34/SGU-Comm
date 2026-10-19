# Desktop app: native notifications

## Summary

Report: "desktop notifications not supported" in the Linux AppImage. The
desktop app uses the web view's Notification API: WebKitGTK asks the app
for permission (we don't answer: refused), Apple's WKWebView has no web
notifications at all, WebView2 (Windows) has them. Use native
notifications (tauri-plugin-notification) on all three; the web client
uses them when it runs in the desktop app.

## Acceptance Criteria

- Pure functions for which notifier is used (desktop app or browser) and
  the permission state, with tests.
- Settings switch works in the desktop app (asks the OS where needed).
- A click brings the app to the front and opens the chat where the
  platform reports clicks; else it only brings the app forward.
- Manual check: macOS build here; Linux and Windows by users.
