# Backups

## What to keep

- The database (`DB_PATH`): accounts, text channels, messages, reactions,
  read state and, with the built-in server, its registrations, channels and
  rights.
- The uploads (`UPLOAD_DIR`): files people sent. They are not in the
  database: back that folder up yourself.
- With the built-in server, its certificate (`cert.pem` and `key.pem` in
  `CERT_DIR`). Without it Mumble clients see a new server and warn.
- Your settings (the compose `.env`, or `/etc/kallisti.env`).

## Database backups

With `BACKUP_DIR` set, Kallisti copies the database:

- at each start, before it opens the database (an update may change it,
  see [updating](updating.md)); if this copy fails, the server does not
  start;
- then every 24 hours while it runs.

The copies are consistent snapshots (SQLite `VACUUM INTO`) named after the
time (`kallisti-20261015-171302.db`). The newest `BACKUP_KEEP` (7) stay.
Copy them to another machine as well: a backup on the same disk does not
help when the disk fails.

## Restore

1. Stop Kallisti.
2. Copy the backup over `DB_PATH`, and remove `DB_PATH-wal` and
   `DB_PATH-shm` if they are there.
3. Start the version of Kallisti that made the backup (after an update: the
   old version, see [updating](updating.md#going-back)).

With Docker, the files are in the `kallisti-data` volume; for example:

```sh
docker compose -f deploy/compose.yml stop kallisti
docker run --rm -v deploy_kallisti-data:/data alpine sh -c \
  'cp /data/backups/kallisti-20261015-171302.db /data/kallisti.db && rm -f /data/kallisti.db-wal /data/kallisti.db-shm'
docker compose -f deploy/compose.yml start kallisti
```

(The volume name starts with the compose project name; `docker volume ls`
shows it.)
