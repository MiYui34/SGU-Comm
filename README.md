# Kallisti

Voice and text chat on top of Mumble, with a Discord-like interface. Web first.
https://kallisti.chat

- `packages/protocol`: Mumble protocol (framing, protobuf codec, state, voice packets)
- `apps/web`: Svelte web client (chat, voice, settings)
- `server`: Go server: web app, accounts, text channels and a built-in Mumble
  server (Mumble clients and web clients in one place)
- `packages/devtools`: dev scripts (smoke test, voice bot)

## Deployment

`deploy/compose.yml` runs the Kallisti server (web app, voice, text channels,
uploads) and Caddy for HTTPS with an automatic certificate:

```sh
printf 'DOMAIN=chat.example.org\nSUPERUSER_PASSWORD=%s\n' "$(openssl rand -hex 16)" > deploy/.env
docker compose -f deploy/compose.yml up -d --remove-orphans
```

Open `https://chat.example.org`. Mumble clients and the desktop app connect to
the same host on port 64738 (TCP and UDP). Kallisti accounts are Mumble
registrations; `SuperUser` (with `SUPERUSER_PASSWORD`) is the admin, for
example to edit permissions with a Mumble client.

Upgrading from a setup with Murmur: `--remove-orphans` stops the old Murmur
container (it would keep port 64738). Murmur's registrations, channels and
ACLs are not taken over.

Bans and limits use the client's IP. Rootless podman rewrites the source
address of native clients (all of them then share one IP); Docker, or
podman with a root network, keeps it.

## Desktop app

`apps/desktop` is a Tauri app around the same web client. A small bridge in
the app (Rust) connects it straight to any Mumble server, without the
Kallisti server; text channels, uploads and accounts need the Kallisti server
and are not shown there. Needs Rust.

```sh
pnpm --filter @kallisti/desktop dev                  # window with the Vite dev server
pnpm --filter @kallisti/desktop build --bundles app  # Kallisti.app
```

## Development

Needs Node with pnpm, Go and bun.

```sh
pnpm install
pnpm test        # TypeScript and Go tests

# Server: web clients on port 8080, Mumble clients on 64738
cd server
ALLOWED_ORIGINS='*' go run ./cmd/server

# Web client on http://localhost:5173 (server defaults to port 8080)
pnpm --filter @kallisti/web dev
```

Server settings: `LISTEN_PORT` (8080), `ALLOWED_ORIGINS` (required,
comma-separated or `*`), `MAX_CONNECTIONS` (100), `MAX_CONNECTIONS_PER_IP`
(10, web connections), `TRUST_FORWARDED_FOR=1` (only behind a reverse proxy
that sets `X-Forwarded-For`), `DB_PATH` (`kallisti.db`), `TEXT_CHANNELS`
(`general`, comma-separated; with the built-in server only the first start
uses it, then admins manage text channels in the client), `UPLOAD_DIR` (`uploads`), `UPLOAD_MAX_MB` (10
per file), `UPLOAD_QUOTA_MB` (100 per user per day), `BACKUP_DIR` (daily
database backups at start and every 24 hours; none when unset),
`BACKUP_KEEP` (7). To restore, stop the server and copy a backup over
`DB_PATH` (remove `DB_PATH-wal` and `DB_PATH-shm` too). Uploads are files in
`UPLOAD_DIR`: back that folder up separately.

The built-in Mumble server: `MUMBLE_LISTEN` (`:64738`, TCP and UDP),
`SERVER_NAME` (`Kallisti`), `MUMBLE_WELCOME`, `MUMBLE_PASSWORD` (for guests),
`MUMBLE_MAX_USERS` (100), `MUMBLE_MAX_PER_IP` (no limit), `CERT_DIR` (the
folder of `DB_PATH`; the server certificate is made on the first start),
`MUMBLED_SUPERUSER_PASSWORD` (sets SuperUser's password),
`MUMBLE_CERT_FILE` and `MUMBLE_KEY_FILE` (PEM files of a certificate for
the server's name, for example from Let's Encrypt, so Mumble clients do not
ask to trust a self-signed one; reloaded when they change; until they exist
the self-signed certificate is used). The files must be readable by the
server's user: Caddy keeps its keys readable by root only, so copy them out
(for example with a daily timer).

With `MUMBLE_HOST` set, the server uses that Murmur instead (`MUMBLE_PORT`,
64738; `MUMBLE_INSECURE_TLS=1` for self-signed certificates).

The official Mumble client in podman, for the server tests:
`KALLISTI_OFFICIAL_CLIENT=1 go test ./internal/mumbled` (see
`packages/devtools/mumble-client`).
The web client reads `VITE_PROXY_URL` to point at another bridge.

Dev scripts (with a running bridge):

```sh
pnpm --filter @kallisti/devtools smoke ws://127.0.0.1:8080
pnpm --filter @kallisti/devtools voicebot tone.opus ws://127.0.0.1:8080
```
