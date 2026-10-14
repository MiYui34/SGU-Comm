# Production deployment with docker-compose

## Summary

A deployable set: Murmur, our server and the static web app. Part of [Sidecar: Discord-style server features](sidecar-discord-style-server-features.md).

## Requirements

- docker-compose file with Murmur and our server; our server also serves the web app.
- HTTPS/WSS (the microphone needs a secure context).
- Content Security Policy. The capture worklet is inlined as a data: URL today, so either allow data: for worklets or emit it as a file.
- Murmur autoban exception for the proxy IP.

## Acceptance Criteria

- `docker compose up` gives a working server with voice and chat over HTTPS.

## Result

- Tested with podman compose and DOMAIN=localhost (HTTPS via Caddy's internal CA). Not tested with a public domain and a real certificate.
