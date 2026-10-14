// Package store keeps text channels and their messages in SQLite.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	_ "modernc.org/sqlite"
)

type Channel struct {
	ID       int64
	Name     string
	Position int
}

type Message struct {
	ID        int64
	ChannelID int64
	Author    string
	Text      string
	CreatedAt time.Time
	// The author was registered on the Mumble server (a verified name).
	Verified bool
	Upload   *Upload // optional attachment
}

// NewMessage is a message to store. UploadID 0: no attachment.
type NewMessage struct {
	ChannelID int64
	Author    string
	Verified  bool
	Text      string
	At        time.Time
	UploadID  int64
}

// Upload is a stored file. Key is the random, unguessable part of its URL.
type Upload struct {
	ID        int64
	Key       string
	Name      string
	Type      string
	Size      int64
	Author    string
	CreatedAt time.Time
	// Used: attached to a message. An upload can be attached once.
	Used bool
}

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS channels (
	id       INTEGER PRIMARY KEY,
	name     TEXT NOT NULL UNIQUE,
	position INTEGER NOT NULL,
	hidden   INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS uploads (
	id         INTEGER PRIMARY KEY,
	key        TEXT NOT NULL UNIQUE,
	name       TEXT NOT NULL,
	type       TEXT NOT NULL,
	size       INTEGER NOT NULL,
	author     TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	used       INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS messages (
	id         INTEGER PRIMARY KEY,
	channel_id INTEGER NOT NULL REFERENCES channels(id),
	author     TEXT NOT NULL,
	text       TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	upload_id  INTEGER REFERENCES uploads(id),
	verified   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS messages_by_channel ON messages(channel_id, id);
`

// Open opens (and creates) the database. ":memory:" is for tests.
func Open(path string) (*Store, error) {
	// synchronous=NORMAL: with WAL still safe against corruption, and an
	// insert does not wait for a full disk sync (the sender's voice waits
	// behind it on the same connection).
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One writer at a time is all SQLite allows anyway, and an in-memory
	// database exists only for one connection.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// migrate brings databases from older versions up to the current schema:
// columns added later are added if missing.
func migrate(db *sql.DB) error {
	for _, col := range []struct{ name, def string }{
		{"upload_id", "INTEGER REFERENCES uploads(id)"},
		{"verified", "INTEGER NOT NULL DEFAULT 0"},
	} {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('messages') WHERE name = ?`, col.name).Scan(&n)
		if err != nil {
			return err
		}
		if n == 0 {
			if _, err := db.Exec(`ALTER TABLE messages ADD COLUMN ` + col.name + ` ` + col.def); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

// EnsureChannels makes the configured channels visible, in this order.
// Channels that are no longer configured are hidden, not deleted, so their
// history comes back if they are configured again.
func (s *Store) EnsureChannels(names []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE channels SET hidden = 1`); err != nil {
		return err
	}
	for i, name := range names {
		_, err := tx.Exec(`INSERT INTO channels (name, position) VALUES (?, ?)
			ON CONFLICT (name) DO UPDATE SET position = excluded.position, hidden = 0`, name, i)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Channels returns the visible channels in order.
func (s *Store) Channels() ([]Channel, error) {
	rows, err := s.db.Query(`SELECT id, name, position FROM channels WHERE hidden = 0 ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.Name, &c.Position); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddMessage stores a message. With an upload, the upload is marked as used
// in the same transaction; a used upload is refused.
func (s *Store) AddMessage(n NewMessage) (Message, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback()
	var upload sql.NullInt64
	if n.UploadID != 0 {
		res, err := tx.Exec(`UPDATE uploads SET used = 1 WHERE id = ? AND used = 0`, n.UploadID)
		if err != nil {
			return Message{}, err
		}
		if rows, _ := res.RowsAffected(); rows != 1 {
			return Message{}, errors.New("upload is missing or already attached")
		}
		upload = sql.NullInt64{Int64: n.UploadID, Valid: true}
	}
	res, err := tx.Exec(`INSERT INTO messages (channel_id, author, verified, text, created_at, upload_id) VALUES (?, ?, ?, ?, ?, ?)`,
		n.ChannelID, n.Author, n.Verified, n.Text, n.At.UnixMilli(), upload)
	if err != nil {
		return Message{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Message{}, err
	}
	m := Message{ID: id, ChannelID: n.ChannelID, Author: n.Author, Verified: n.Verified, Text: n.Text, CreatedAt: n.At}
	if upload.Valid {
		u, err := scanUpload(tx.QueryRow(`SELECT `+uploadColumns+` FROM uploads WHERE id = ?`, n.UploadID))
		if err != nil {
			return Message{}, err
		}
		m.Upload = &u
	}
	return m, tx.Commit()
}

func (s *Store) AddUpload(u Upload, at time.Time) (Upload, error) {
	res, err := s.db.Exec(`INSERT INTO uploads (key, name, type, size, author, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		u.Key, u.Name, u.Type, u.Size, u.Author, at.UnixMilli())
	if err != nil {
		return Upload{}, err
	}
	u.ID, err = res.LastInsertId()
	u.CreatedAt = at
	return u, err
}

func (s *Store) UploadByKey(key string) (Upload, error) {
	return scanUpload(s.db.QueryRow(`SELECT `+uploadColumns+` FROM uploads WHERE key = ?`, key))
}

const uploadColumns = `id, key, name, type, size, author, created_at, used`

func scanUpload(row interface{ Scan(...any) error }) (Upload, error) {
	var u Upload
	var at int64
	err := row.Scan(&u.ID, &u.Key, &u.Name, &u.Type, &u.Size, &u.Author, &at, &u.Used)
	u.CreatedAt = time.UnixMilli(at).UTC()
	return u, err
}

// History returns up to limit messages before beforeID (0: the newest),
// oldest first, with their attachments.
func (s *Store) History(channelID, beforeID int64, limit int) ([]Message, error) {
	rows, err := s.db.Query(`SELECT m.id, m.channel_id, m.author, m.verified, m.text, m.created_at,
			u.id, u.key, u.name, u.type, u.size, u.author, u.created_at, u.used
		FROM messages m LEFT JOIN uploads u ON u.id = m.upload_id
		WHERE m.channel_id = ? AND (? = 0 OR m.id < ?) ORDER BY m.id DESC LIMIT ?`,
		channelID, beforeID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var at int64
		var u struct {
			id, size, at        sql.NullInt64
			key, name, typ, who sql.NullString
			used                sql.NullBool
		}
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.Author, &m.Verified, &m.Text, &at,
			&u.id, &u.key, &u.name, &u.typ, &u.size, &u.who, &u.at, &u.used); err != nil {
			return nil, err
		}
		m.CreatedAt = time.UnixMilli(at).UTC()
		if u.id.Valid {
			m.Upload = &Upload{ID: u.id.Int64, Key: u.key.String, Name: u.name.String, Type: u.typ.String,
				Size: u.size.Int64, Author: u.who.String, CreatedAt: time.UnixMilli(u.at.Int64).UTC(), Used: u.used.Bool}
		}
		out = append(out, m)
	}
	slices.Reverse(out)
	return out, rows.Err()
}

// DeleteUnusedUploads removes uploads created before `before` that were
// never attached, and returns their keys (the caller deletes the files).
func (s *Store) DeleteUnusedUploads(before time.Time) ([]string, error) {
	rows, err := s.db.Query(`DELETE FROM uploads WHERE used = 0 AND created_at < ? RETURNING key`, before.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
