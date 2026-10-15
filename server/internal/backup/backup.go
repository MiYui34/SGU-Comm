// Package backup makes consistent copies of the SQLite database while the
// server runs, and keeps the newest few.
package backup

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const pattern = "kallisti-*.db"

// Run writes a copy named after the time into dir (VACUUM INTO: a
// consistent snapshot) and removes all but the newest keep copies.
func Run(db *sql.DB, dir string, keep int, now time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "kallisti-"+now.UTC().Format("20060102-150405")+".db")
	if _, err := db.Exec(`VACUUM INTO ?`, path); err != nil {
		return "", fmt.Errorf("backup to %s: %w", path, err)
	}
	old, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return path, err
	}
	slices.Sort(old) // the names sort by time
	for _, f := range old[:max(0, len(old)-keep)] {
		if err := os.Remove(f); err != nil {
			slog.Warn("remove old backup", "file", f, "err", err)
		}
	}
	return path, nil
}

// Daily runs a backup now and then every day, until the process ends.
func Daily(db *sql.DB, dir string, keep int) {
	for {
		if path, err := Run(db, dir, keep, time.Now()); err != nil {
			slog.Error("database backup", "err", err)
		} else {
			slog.Info("database backup", "file", path)
		}
		time.Sleep(24 * time.Hour)
	}
}
