# Sidecar: Discord-style server features

## Summary

Meta issue. Our server (today `packages/proxy`) grows into the sidecar next to Murmur: text channels, history, uploads and identity. A deployment is Murmur + our server + the web app. See DEVLOG.md, "Architecture direction".

## Requirements

- Done when all sub-issues are done:
- [Rewrite the server in Rust or Go](rewrite-the-server-in-rust-or-go.md)
- [Text channels with persistent history](text-channels-with-persistent-history.md)
- [File and image uploads](file-and-image-uploads.md)
- [User accounts and registration behind the proxy](user-accounts-and-registration.md)
- [Production deployment with docker-compose](production-deployment-with-docker-compose.md)

## Acceptance Criteria

- All sub-issues are archived.

## Notes

- The protocol package and the voice code must not depend on the sidecar. A plain Mumble server must keep working without it.
- Desktop Mumble users keep voice and in-room chat only.
