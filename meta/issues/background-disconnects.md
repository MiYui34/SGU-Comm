# Disconnects when the app is in the background

## Summary

Reports: the macOS desktop app disconnects as soon as it leaves focus;
on Android (PWA in Chromium/Brave) switching apps or the screen going
dark drops the connection and the call. Cause: the client has no
watchdog of its own; the server's bridge pings the browser at WebSocket
level and closes it after 30 s without an answer. A suspended web process
(macOS WKWebView inactive scheduling + App Nap; Android freezing pages)
cannot answer.

Fixes:
1. macOS app: WKWebView inactiveSchedulingPolicy = none (macOS 14+) and
   no App Nap while connected (NSProcessInfo activity).
2. Server bridge: a longer grace for unanswered browser pings (~2 min) so
   short switches survive; voice and text catch up after.
3. Android/web: during a call, play voice through an <audio> element with
   a Media Session (title, leave action) so the page counts as playing
   media and is kept alive; a quick silent rejoin after a reconnect.

## Acceptance Criteria

- Go tests for the bridge grace; Rust compiles for macOS (no GUI test on
  the dev Mac); vitest for the media-session/rejoin logic.
- Manual: the user (macOS app in the background) and a Linux/Android user.
