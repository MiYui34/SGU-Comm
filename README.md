# mumble-discord

A Mumble client with a Discord-like interface. Web first. Voice is not done yet.

- `packages/protocol`: Mumble protocol (framing, protobuf codec, state)
- `packages/proxy`: WebSocket to Mumble TLS bridge
- `apps/web`: Svelte web client

## Development

```sh
pnpm install
pnpm test

# A local Mumble server
podman run -d --name md-murmur -p 64738:64738 docker.io/mumblevoip/mumble-server:latest

# Proxy (most Mumble servers use self-signed certificates)
MUMBLE_HOST=127.0.0.1 MUMBLE_INSECURE_TLS=1 ALLOWED_ORIGINS='*' \
  pnpm --filter @mumble-discord/proxy start

# Web client on http://localhost:5173 (proxy defaults to port 8080)
pnpm --filter @mumble-discord/web dev
```

Proxy settings: `MUMBLE_HOST` (required), `MUMBLE_PORT` (64738),
`LISTEN_PORT` (8080), `ALLOWED_ORIGINS` (required, comma-separated or `*`),
`MUMBLE_INSECURE_TLS=1`, `MAX_CONNECTIONS` (100).
The web client reads `VITE_PROXY_URL` to point at another proxy.
