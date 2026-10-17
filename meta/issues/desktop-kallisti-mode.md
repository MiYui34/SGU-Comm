# Desktop: Kallisti mode

## Summary

The desktop app asks the address over HTTPS whether it is a Kallisti
server. If yes, it runs the web client against it (accounts API, the
WebSocket with voice and text, uploads, images) with an access token;
if not, the Mumble bridge as today. The CSP allows https/wss to servers
and their images and media.

## Acceptance Criteria

- Unit tests for the detection and the URLs; manual check of the desktop
  app against a local Kallisti server (log in with a password, text
  channels, voice) and against a plain Mumble server.
