# Kallisti

Voice and text chat on top of Mumble, with a Discord-like interface. Web first.
https://kallisti.chat

- `packages/protocol`: Mumble protocol (framing, protobuf codec, state, voice packets)
- `apps/web`: Svelte web client (chat, voice, settings)
- `server`: Go server that bridges browser WebSockets to the Mumble TLS port
- `packages/devtools`: dev scripts (smoke test, voice bot)

## Deployment

`deploy/compose.yml` runs Murmur, the Kallisti server (web app, bridge, text
channels, uploads) and Caddy for HTTPS with an automatic certificate:

```sh
echo "DOMAIN=chat.example.org" > deploy/.env
docker compose -f deploy/compose.yml up -d
```

Open `https://chat.example.org`. Desktop Mumble clients connect to the same
host on port 64738. Murmur's autoban is off: all web users reach Murmur from
the Kallisti container, and the Kallisti server limits connections per client
IP instead.

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
pnpm --filter @kallisti/web dev
```

Server settings: `MUMBLE_HOST` (required), `MUMBLE_PORT` (64738),
`LISTEN_PORT` (8080), `ALLOWED_ORIGINS` (required, comma-separated or `*`),
`MUMBLE_INSECURE_TLS=1`, `MAX_CONNECTIONS` (100), `MAX_CONNECTIONS_PER_IP` (10),
`TRUST_FORWARDED_FOR=1` (only behind a reverse proxy that sets
`X-Forwarded-For`), `DB_PATH` (`kallisti.db`), `TEXT_CHANNELS` (`general`,
comma-separated), `UPLOAD_DIR` (`uploads`), `UPLOAD_MAX_MB` (10 per file),
`UPLOAD_QUOTA_MB` (100 per user per day).
The web client reads `VITE_PROXY_URL` to point at another bridge.

Dev scripts (with a running bridge):

```sh
pnpm --filter @kallisti/devtools smoke ws://127.0.0.1:8080
pnpm --filter @kallisti/devtools voicebot tone.opus ws://127.0.0.1:8080
```
