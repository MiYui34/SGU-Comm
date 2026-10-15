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
