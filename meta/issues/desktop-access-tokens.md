# Server: access tokens for the desktop app

## Summary

The web app logs in with an HttpOnly cookie. The desktop app's page has
another origin (tauri://localhost, http://tauri.localhost), where the
server's cookie is third-party (blocked by WebKit). Login, guest accounts
and the account API also return and accept an access token
(Authorization: Bearer, and on the WebSocket a token parameter); the
Tauri origins are always allowed, with tokens only (no cookies, so no
CSRF).

## Acceptance Criteria

- Server tests: token from login/guest/create, used by the API, the
  WebSocket and uploads; logout ends it; the Tauri origins get CORS
  without credentials; cookies keep working for browsers.
