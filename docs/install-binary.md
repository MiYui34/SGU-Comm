# Installing without containers

Kallisti is one Go program plus the built web app (a folder of files). It
needs no database server: the data is one SQLite file. You need a reverse
proxy for HTTPS in front of it (Caddy below; nginx works too).

## Build

You need Node 24 with pnpm 11 and Go 1.26 (the `Dockerfile` shows the exact
versions it builds with). On the build machine:

```sh
git clone https://github.com/lambadalambda/kallisti.git
cd kallisti
pnpm install --frozen-lockfile --filter @kallisti/web...
pnpm --filter @kallisti/web build                       # -> apps/web/dist
cd server
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../kallisti ./cmd/server
```

The program has no other dependencies (no cgo), so you can build it on
another machine: add `GOOS=linux GOARCH=amd64` (or `arm64`) for the server.

## Install

On the server, as root:

```sh
useradd --system --home /var/lib/kallisti --shell /usr/sbin/nologin kallisti
mkdir -p /opt/kallisti
cp kallisti /opt/kallisti/kallisti
cp -r apps/web/dist /opt/kallisti/web
cp deploy/kallisti.service /etc/systemd/system/
```

Settings go into `/etc/kallisti.env` (readable by root only: it holds the
SuperUser password). A minimal file for the built-in Mumble server:

```sh
ALLOWED_ORIGINS=https://chat.example.org
TRUST_FORWARDED_FOR=1
WEB_DIR=/opt/kallisti/web
DB_PATH=/var/lib/kallisti/kallisti.db
UPLOAD_DIR=/var/lib/kallisti/uploads
BACKUP_DIR=/var/lib/kallisti/backups
LISTEN_PORT=8080
MUMBLED_SUPERUSER_PASSWORD=choose-a-long-password
```

All settings: [configuration](configuration.md). For an existing Murmur, add
`MUMBLE_HOST` (and see [the Docker page](install-docker.md#in-front-of-an-existing-murmur)
for what to set on Murmur).

```sh
chmod 600 /etc/kallisti.env
systemctl daemon-reload
systemctl enable --now kallisti
journalctl -u kallisti -f      # "server running" when it is up
```

The service file runs Kallisti as the `kallisti` user with its data in
`/var/lib/kallisti` (systemd makes the folder) and keeps it from changing
the rest of the system.

## HTTPS with Caddy

`/etc/caddy/Caddyfile`:

```
chat.example.org {
	reverse_proxy 127.0.0.1:8080
}
```

Caddy gets the certificate by itself. WebSockets (the voice and chat
connection) pass through without extra settings. Let Kallisti listen only
on localhost if the proxy is on the same machine: a firewall rule for port
8080, or put the proxy and Kallisti on a private network.

## The Mumble port

With the built-in server, Mumble clients and the desktop app connect to port
64738 (TCP and UDP) directly, not through the proxy: open it in the
firewall.

They warn about a self-signed certificate once. To use the web certificate
instead, copy Caddy's files for the domain where Kallisti can read them and
set `MUMBLE_CERT_FILE` and `MUMBLE_KEY_FILE`. Kallisti reloads them when they
change. Caddy keeps its files readable by its own user only, so copy them
regularly, for example with a daily systemd timer that runs:

```sh
src=/var/lib/caddy/.local/share/caddy/certificates/acme-v02.api.letsencrypt.org-directory/chat.example.org
install -o kallisti -m 600 "$src/chat.example.org.crt" /var/lib/kallisti/mumble.crt
install -o kallisti -m 600 "$src/chat.example.org.key" /var/lib/kallisti/mumble.key
```

## Updates and backups

See [updating](updating.md#without-containers) and [backups](backups.md).
