# Configuration

Kallisti reads its settings from environment variables. Defaults are in
brackets.

- With the systemd service: `/etc/kallisti.env`.
- With compose: only the variables in the `environment:` section of the
  compose file reach Kallisti (some of them take their value from
  `deploy/.env`). A setting that is only in `deploy/.env` does nothing: add
  it to `environment:` too.
- In `deploy/.env`, compose reads `$` as the start of a variable: put
  values with `$` in single quotes (`SUPERUSER_PASSWORD='pa$$word'`).

## Web and general

- `ALLOWED_ORIGINS` (required): the address of the web app, for example
  `https://chat.example.org`; comma-separated for several, `*` only for
  development.
- `LISTEN_PORT` (8080): the web app and the web clients' connections.
- `WEB_DIR` (none): the built web app (`apps/web/dist`). Without it the
  server only answers web clients' connections (development with Vite).
- `TRUST_FORWARDED_FOR=1`: use the last `X-Forwarded-For` address (the one
  your reverse proxy adds) as the client's IP. Only with exactly one proxy
  in front that sets it: clients can fake the header. Caddy replaces the
  header and nginx with `$proxy_add_x_forwarded_for` adds to it; both work.
  With two proxies (for example Cloudflare and nginx), all users would get
  the first proxy's address.
- `MAX_CONNECTIONS` (100) and `MAX_CONNECTIONS_PER_IP` (10): web connections. Per-address limits
  (also login attempts) count an IPv6 /64 as one address: devices on one
  network that share a /64 share the limit.
- `DB_PATH` (`kallisti.db`): the SQLite database.
- `TEXT_CHANNELS` (`general`, comma-separated): with the built-in server
  only the first start uses it, then admins manage text channels in the app;
  in front of a Murmur it is read at each start.
- `UPLOAD_DIR` (`uploads`), `UPLOAD_MAX_MB` (10 per file),
  `UPLOAD_QUOTA_MB` (100 per user per day).
- `BACKUP_DIR` (none: no backups) and `BACKUP_KEEP` (7): see
  [backups](backups.md).
- `LINK_PREVIEWS=0`: the server fetches no pages for link previews. By
  default it fetches the first link of a message, only from public
  addresses, so linked sites see the server's address.

## The built-in Mumble server

Used unless `MUMBLE_HOST` is set.

- `MUMBLE_LISTEN` (`:64738`, TCP and UDP): Mumble clients and the desktop
  app.
- `SERVER_NAME` (`Kallisti`), `MUMBLE_WELCOME` (the welcome text).
- `MUMBLE_PASSWORD`: a server password for guests. With it, saved avatars
  are not served to everyone (the server is private).
- `MUMBLE_MAX_USERS` (100), `MUMBLE_MAX_PER_IP` (no limit).
- `MUMBLED_SUPERUSER_PASSWORD`: sets SuperUser's password at the start
  (unset: the password from an earlier start stays).
- `CERT_DIR` (the folder of `DB_PATH`): the self-signed server certificate,
  made on the first start.
- `MUMBLE_CERT_FILE` and `MUMBLE_KEY_FILE`: PEM files of a certificate for
  the server's name (for example Let's Encrypt), so Mumble clients do not ask
  to trust a self-signed one. Reloaded when they change; until they exist the
  self-signed certificate is used. They must be readable by the server's
  user.

### Streams from OBS (WHIP)

Users stream from OBS 30 or newer (or another WHIP encoder) into their voice
channel; it shows like a screen share. Each user makes a stream key in
Settings → "Stream from OBS", which also shows the server address
(`https://<your domain>/api/whip`) and the OBS settings. Off unless
`WHIP_UDP_PORT` is set (`deploy/compose.yml` sets it).

- `WHIP_UDP_PORT` (off): one UDP port for all streams (8189 in the compose
  file). Open it in the firewall. No TURN or STUN server is needed.
- `WHIP_PUBLIC_IP`: the server's public IPv4 address (or a name, resolved at
  the start), sent to OBS as the address to reach. Needed where the server
  does not see it, as in Docker (the compose file uses `DOMAIN`). Behind
  Cloudflare's proxy, set the IP address itself. Unset: the addresses of the
  server's interfaces.
- Only IPv4, only UDP (OBS has no ICE over TCP).
- A stream needs a registered account (no guest accounts), connected to
  voice in a channel where it may speak. It ends when OBS stops, the
  connection fails, or the user leaves the channel or disconnects.
- OBS must send H.264 (packetization-mode 1) and Opus. The server forwards
  the video as it comes: viewers need a browser that decodes H.264, and the
  stream should have no B-frames, a keyframe every 1 or 2 seconds, and
  at most 4096 pixels per side (larger streams are refused). Pictures above
  8 Mbit/s are dropped; at most 20 streams from OBS at once.

## In front of an existing Murmur

- `MUMBLE_HOST`: the Murmur server (this turns the built-in server off).
- `MUMBLE_PORT` (64738).
- `MUMBLE_INSECURE_TLS=1`: accept Murmur's self-signed certificate. Off by
  default (the certificate is checked); `compose.murmur.yml` sets 1. Most
  Murmur servers have a self-signed certificate.

## Development

- `VITE_PROXY_URL` (web client, build time): the WebSocket address of another
  server (`wss://host/ws`); the API address comes from it.
