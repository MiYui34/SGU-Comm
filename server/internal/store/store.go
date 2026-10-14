// Package store keeps text channels and their messages in SQLite.
package store

import (
	"database/sql"
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
}

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS channels (
	id       INTEGER PRIMARY KEY,
	name     TEXT NOT NULL UNIQUE,
	position INTEGER NOT NULL,
	hidden   INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS messages (
	id         INTEGER PRIMARY KEY,
	channel_id INTEGER NOT NULL REFERENCES channels(id),
	author     TEXT NOT NULL,
	text       TEXT NOT NULL,
	created_at INTEGER NOT NULL
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
	return &Store{db: db}, nil
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

func (s *Store) AddMessage(channelID int64, author, text string, at time.Time) (Message, error) {
	res, err := s.db.Exec(`INSERT INTO messages (channel_id, author, text, created_at) VALUES (?, ?, ?, ?)`,
		channelID, author, text, at.UnixMilli())
	if err != nil {
		return Message{}, err
	}
	id, err := res.LastInsertId()
	return Message{ID: id, ChannelID: channelID, Author: author, Text: text, CreatedAt: at}, err
}

// History returns up to limit messages before beforeID (0: the newest),
// oldest first.
func (s *Store) History(channelID, beforeID int64, limit int) ([]Message, error) {
	rows, err := s.db.Query(`SELECT id, channel_id, author, text, created_at FROM messages
		WHERE channel_id = ? AND (? = 0 OR id < ?) ORDER BY id DESC LIMIT ?`,
		channelID, beforeID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var at int64
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.Author, &m.Text, &at); err != nil {
			return nil, err
		}
		m.CreatedAt = time.UnixMilli(at).UTC()
		out = append(out, m)
	}
	slices.Reverse(out)
	return out, rows.Err()
}
