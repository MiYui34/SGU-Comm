package store

import (
	"errors"
	"strings"
	"time"
)

// MaxEmoji is the most custom emoji a server has.
const MaxEmoji = 200

var ErrTooManyEmoji = errors.New("too many custom emoji")

// Emoji is a custom server emoji: a name and a small image. IDs are never
// used again (AUTOINCREMENT), so an image URL by id can be cached.
type Emoji struct {
	ID        int64
	Name      string
	Type      string
	Data      []byte // only from EmojiByID
	Author    string
	CreatedAt time.Time
}

const emojiTable = `CREATE TABLE IF NOT EXISTS emoji (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL UNIQUE,
	type       TEXT NOT NULL,
	data       BLOB NOT NULL,
	author     TEXT NOT NULL,
	created_at INTEGER NOT NULL
)`

// AddEmoji stores a custom emoji (the caller checks name and image).
func (s *Store) AddEmoji(e Emoji, at time.Time) (Emoji, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Emoji{}, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM emoji`).Scan(&n); err != nil {
		return Emoji{}, err
	}
	if n >= MaxEmoji {
		return Emoji{}, ErrTooManyEmoji
	}
	err = tx.QueryRow(`INSERT INTO emoji (name, type, data, author, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		e.Name, e.Type, e.Data, e.Author, at.UnixMilli()).Scan(&e.ID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Emoji{}, ErrNameTaken
		}
		return Emoji{}, err
	}
	e.CreatedAt = at
	return e, tx.Commit()
}

// Emojis lists the custom emoji by name, without their images.
func (s *Store) Emojis() ([]Emoji, error) {
	rows, err := s.db.Query(`SELECT id, name, type, author, created_at FROM emoji ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Emoji
	for rows.Next() {
		var e Emoji
		var at int64
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Author, &at); err != nil {
			return nil, err
		}
		e.CreatedAt = time.UnixMilli(at).UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

// EmojiByID is a custom emoji with its image.
func (s *Store) EmojiByID(id int64) (Emoji, error) {
	var e Emoji
	var at int64
	err := s.db.QueryRow(`SELECT id, name, type, data, author, created_at FROM emoji WHERE id = ?`, id).
		Scan(&e.ID, &e.Name, &e.Type, &e.Data, &e.Author, &at)
	e.CreatedAt = time.UnixMilli(at).UTC()
	return e, err
}

func (s *Store) DeleteEmoji(name string) error {
	res, err := s.db.Exec(`DELETE FROM emoji WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
