# mumble-discord

A Mumble client with a Discord-like interface. Web first.

- `packages/protocol`: Mumble protocol (framing, protobuf codec, state, voice packets)
- `apps/web`: Svelte web client (chat, voice, settings)
- `server`: Go server that bridges browser WebSockets to the Mumble TLS port
- `packages/devtools`: dev scripts (smoke test, voice bot)

## Development

Needs Node with pnpm, Go and bun.

```sh
pnpm install
pnpm test        # TypeScript and Go tests

# A local Mumble server
podman run -d --name md-murmur -p 64738:64738 -p 64738:64738/udp \
  docker.io/mumblevoip/mumble-server:latest

# Bridge (most Mumble servers use self-signed certificates)
cd server
MUMBLE_HOST=127.0.0.1 MUMBLE_INSECURE_TLS=1 ALLOWED_ORIGINS='*' go run ./cmd/server

# Web client on http://localhost:5173 (bridge defaults to port 8080)
pnpm --filter @mumble-discord/web dev
```

Server settings: `MUMBLE_HOST` (required), `MUMBLE_PORT` (64738),
`LISTEN_PORT` (8080), `ALLOWED_ORIGINS` (required, comma-separated or `*`),
`MUMBLE_INSECURE_TLS=1`, `MAX_CONNECTIONS` (100).
The web client reads `VITE_PROXY_URL` to point at another bridge.

Dev scripts (with a running bridge):

```sh
pnpm --filter @mumble-discord/devtools smoke ws://127.0.0.1:8080
pnpm --filter @mumble-discord/devtools voicebot tone.opus ws://127.0.0.1:8080
```
