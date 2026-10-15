# Configuration

Kallisti reads its settings from environment variables (in the compose
files, in `/etc/kallisti.env` for the systemd service). Defaults are in
brackets.

## Web and general

- `ALLOWED_ORIGINS` (required): the address of the web app, for example
  `https://chat.example.org`; comma-separated for several, `*` only for
  development.
- `LISTEN_PORT` (8080): the web app and the web clients' connections.
- `WEB_DIR` (none): the built web app (`apps/web/dist`). Without it the
  server only answers web clients' connections (development with Vite).
- `TRUST_FORWARDED_FOR=1`: use the first `X-Forwarded-For` address as the
  client's IP. Only behind a reverse proxy that sets it: clients can fake it.
- `MAX_CONNECTIONS` (100) and `MAX_CONNECTIONS_PER_IP` (10): web connections.
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
- `MUMBLE_PASSWORD`: a server password for guests.
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

## In front of an existing Murmur

- `MUMBLE_HOST`: the Murmur server (this turns the built-in server off).
- `MUMBLE_PORT` (64738).
- `MUMBLE_INSECURE_TLS=1`: accept Murmur's self-signed certificate (set 0 if
  it has a real one).

## Development

- `VITE_PROXY_URL` (web client, build time): another server to connect to.
