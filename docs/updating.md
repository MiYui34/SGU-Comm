# Updating

There are no releases yet: an update means the newest `main` from git.

## What happens at an update

At its start, a new version brings the database up to date by itself
(migrations). It needs no manual steps, but the changes are one-way: an
older version may not work with the database afterwards. So:

- Set `BACKUP_DIR` (the compose files do). Kallisti then copies the
  database before it opens it, so there is always a copy from just before
  the update. If that copy fails (for example, a full disk), the server
  does not start.
- Read the [upgrade notes](#upgrade-notes) below for changes you should
  know about.
- After the start, look at the log: "database backup before the start"
  (the copy) and "server running".

Uploads and the server certificate do not change at updates.

## Docker or Podman

```sh
git pull
docker compose -f deploy/compose.yml up -d --build
docker compose -f deploy/compose.yml logs -f kallisti
```

(With your compose file: `compose.behind-proxy.yml` or
`compose.murmur.yml`.) The data stays in the volume.

## Without containers

Build as in [installing](install-binary.md#build), then on the server:

```sh
systemctl stop kallisti
cp kallisti /opt/kallisti/kallisti
rm -rf /opt/kallisti/web && cp -r apps/web/dist /opt/kallisti/web
systemctl start kallisti
journalctl -u kallisti -f
```

Stop first: a running program cannot be replaced on every system. Replace
the web app folder as a whole: old asset files may stay, but the new
`index.html` must come with the new program.

## Going back

1. Stop Kallisti.
2. Restore the database copy from before the update ([backups](backups.md#restore)).
3. Start the old version (`git checkout <old commit>`, then build and
   install it as above).

Messages and other changes made with the new version are lost with the old
database.

## Upgrade notes

Newest first. Each migration runs once, at the first start of a version
that has it.

### 2026-10-15

- **User ids** (built-in server): message ownership, read marks and
  reactions now use the Mumble user id, not the name. The migration finds
  the ids by name. Verified messages whose author no longer has a
  registration lose the verified mark (only moderators can then edit or
  delete them). In front of a Murmur, read marks start again from the
  beginning once (there are no ids to find); old messages keep their owner
  by name.
- **Guest accounts** (built-in server): "Join as guest" creates a temporary
  account, removed after a day without a visit. Registration ids are never
  given twice from now on.
- **Reactions**, **link previews** (the server fetches pages: see
  `LINK_PREVIEWS`), **typing indicator**, **Markdown**.
- **Noise suppression** in the web app (RNNoise, DeepFilterNet3). The
  Content Security Policy allows WebAssembly (`wasm-unsafe-eval`). The
  server now gzips the web app's files.
- **Backups** are now made before the start, then daily (before: at the
  start, after the migrations).
- Earlier the same day: message search (a full-text index, built once
  from all messages; this can take a moment on a large database), replies,
  topics, read state, editing and deleting messages, moderators, text
  channel mirrors for Mumble clients, the lobby, avatars.
