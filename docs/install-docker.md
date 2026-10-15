# Installing with Docker or Podman

The compose files in `deploy/` run Kallisti and Caddy, which gets the HTTPS
certificate by itself. Browsers allow the microphone only on secure pages,
so Kallisti needs a domain with HTTPS.

## Built-in Mumble server (recommended)

`deploy/compose.yml` runs Kallisti (web app, voice, text channels, uploads)
and Caddy:

```sh
printf 'DOMAIN=chat.example.org\nSUPERUSER_PASSWORD=%s\n' "$(openssl rand -hex 16)" > deploy/.env
docker compose -f deploy/compose.yml up -d --remove-orphans
```

- Open `https://chat.example.org`. Mumble clients and the desktop app
  connect to the same host on port 64738 (TCP and UDP): open it in your
  firewall.
- `SuperUser` is the admin: log in with the "Log in" tab (or a Mumble
  client) and `SUPERUSER_PASSWORD`. Admins make moderators in a user's menu.
- Mumble clients warn about the self-signed certificate once. To avoid
  that, give the server your web certificate (`MUMBLE_CERT_FILE`, see
  [configuration](configuration.md#the-built-in-mumble-server)).
- Behind a reverse proxy that already runs in Docker, use
  `deploy/compose.behind-proxy.yml`: set `PROXY_NETWORK` to the proxy's
  Docker network and point the proxy at `kallisti:8080`.
- Coming from Murmur: `--remove-orphans` stops an old Murmur container (it
  would keep port 64738). Murmur's registrations, channels and ACLs are not
  taken over.

## In front of an existing Murmur

`deploy/compose.murmur.yml` runs Kallisti and Caddy, without a Mumble port:

```sh
printf 'DOMAIN=chat.example.org\nMUMBLE_HOST=voice.example.org\n' > deploy/.env
docker compose -f deploy/compose.murmur.yml up -d
```

- Each web user gets their own Mumble session on your Murmur, made by the
  Kallisti server. Murmur therefore sees all web users at the Kallisti
  server's address: set `autobanAttempts=0` in `murmur.ini`, or Murmur may
  ban Kallisti after a few reconnects.
- Accounts register themselves on Murmur with their own certificate (Murmur
  allows that by default). The admin stays Murmur's SuperUser: join with the
  "Guest" tab, the name `SuperUser` and the SuperUser password as the server
  password.
- `TEXT_CHANNELS` is the list of text channels; it is read at each start.

## Data

Everything lives in the `kallisti-data` volume (`/data` in the container):
the database, uploads, database backups (`/data/backups`) and, with the
built-in server, its certificate. See [backups](backups.md).

Bans and limits use the client's IP. Rootless podman rewrites the source
address of native clients (all of them then share one IP); Docker, or podman
with a root network, keeps it.

## Updates

See [updating](updating.md#docker-or-podman).
