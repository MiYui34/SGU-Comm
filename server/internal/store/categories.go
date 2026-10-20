package store

import (
	"database/sql"
	"errors"
	"strings"

	"kallisti.chat/server/internal/names"
)

// Category groups text channels (one level, like Discord's). Channels
// without one (Channel.Category 0) come first.
type Category struct {
	ID       int64
	Name     string
	Position int
	// Mumble is the channel that holds its channels' mirrors (0: none).
	Mumble uint32
}

// name_key: names.Key, as Mumble compares sibling channel names.
const categoriesTable = `CREATE TABLE IF NOT EXISTS categories (
	id             INTEGER PRIMARY KEY,
	name           TEXT NOT NULL,
	name_key       TEXT NOT NULL UNIQUE,
	position       INTEGER NOT NULL,
	mumble_channel INTEGER
)`

// nameTaken maps a unique constraint error to ErrNameTaken.
func nameTaken(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return ErrNameTaken
	}
	return err
}

// rowChanged: ErrNotFound when the statement changed no row.
func rowChanged(res sql.Result, err error) error {
	if err != nil {
		return nameTaken(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Categories() ([]Category, error) {
	rows, err := s.db.Query(`SELECT id, name, position, COALESCE(mumble_channel, 0) FROM categories ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Position, &c.Mumble); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCategory adds a category at the end.
func (s *Store) CreateCategory(name string) (Category, error) {
	c := Category{Name: name}
	err := s.db.QueryRow(`INSERT INTO categories (name, name_key, position)
		VALUES (?, ?, (SELECT COALESCE(MAX(position), -1) + 1 FROM categories)) RETURNING id, position`,
		name, names.Key(name)).Scan(&c.ID, &c.Position)
	return c, nameTaken(err)
}

func (s *Store) RenameCategory(id int64, name string) error {
	return rowChanged(s.db.Exec(`UPDATE categories SET name = ?, name_key = ? WHERE id = ?`, name, names.Key(name), id))
}

// DeleteCategory removes a category; its channels (also hidden ones) are
// uncategorized after it.
func (s *Store) DeleteCategory(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE channels SET category_id = NULL WHERE category_id = ?`, id); err != nil {
		return err
	}
	if err := rowChanged(tx.Exec(`DELETE FROM categories WHERE id = ?`, id)); err != nil {
		return err
	}
	return tx.Commit()
}

// SetCategoryPositions puts the given categories in this order.
func (s *Store) SetCategoryPositions(ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE categories SET position = ? WHERE id = ?`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetCategoryMumble remembers the Mumble channel of a category.
func (s *Store) SetCategoryMumble(id int64, mumble uint32) error {
	_, err := s.db.Exec(`UPDATE categories SET mumble_channel = ? WHERE id = ?`, mumble, id)
	return err
}

// SetChannelCategory moves a text channel into a category (0: none), as
// its last channel.
func (s *Store) SetChannelCategory(channel, category int64) error {
	if err := s.categoryExists(category); err != nil {
		return err
	}
	return s.textChannel(`UPDATE channels SET category_id = NULLIF(?, 0),
		position = (SELECT COALESCE(MAX(position), -1) + 1 FROM channels WHERE hidden = 0)
		WHERE id = ? AND voice_channel IS NULL AND dm_low IS NULL`, category, channel)
}

// categoryExists: nil for 0 (none) and for a category there is.
func (s *Store) categoryExists(id int64) error {
	if id == 0 {
		return nil
	}
	err := s.db.QueryRow(`SELECT id FROM categories WHERE id = ?`, id).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
