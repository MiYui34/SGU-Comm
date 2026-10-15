# Automatic database backups

## Summary

The server makes a consistent SQLite backup on a schedule and keeps the
last few.

## Requirements

- Settings: BACKUP_DIR (off when unset), BACKUP_KEEP (default 7), once a
  day and at start.
- Consistent while running (VACUUM INTO); old backups removed.
- README documents it and how to restore.

## Acceptance Criteria

- Tests for backup and pruning.

## Result

- BACKUP_DIR / BACKUP_KEEP; a VACUUM INTO copy at start and every 24
  hours; old copies removed (only kallisti-*.db files). README explains
  restoring; deploy/compose.yml turns it on in the data volume.
- Tests: backup and pruning, settings.
