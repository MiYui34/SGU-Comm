package mumbled

import (
	"database/sql"
	"errors"

	"kallisti.chat/server/internal/names"
)

// DB stores the Mumble server's data in the Kallisti server's SQLite file.
type DB struct{ db *sql.DB }

const dbSchema = `
CREATE TABLE IF NOT EXISTS mumble_channels (
	id          INTEGER PRIMARY KEY,
	parent      INTEGER NOT NULL,
	name        TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	position    INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS mumble_meta (
	key   TEXT PRIMARY KEY,
	value INTEGER NOT NULL
);
-- Root has a row too: its description, and links to it.
INSERT OR IGNORE INTO mumble_channels (id, parent, name) VALUES (0, 0, 'Root');
CREATE TABLE IF NOT EXISTS mumble_users (
	id            INTEGER PRIMARY KEY,
	name          TEXT NOT NULL,
	name_key      TEXT NOT NULL UNIQUE,
	cert_hash     TEXT UNIQUE,
	password_hash TEXT NOT NULL DEFAULT '',
	last_channel  INTEGER NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO mumble_users (id, name, name_key) VALUES (0, 'SuperUser', 'superuser');
CREATE TABLE IF NOT EXISTS mumble_channel_links (
	a INTEGER NOT NULL REFERENCES mumble_channels(id) ON DELETE CASCADE,
	b INTEGER NOT NULL REFERENCES mumble_channels(id) ON DELETE CASCADE,
	PRIMARY KEY (a, b)
);
`

func OpenDB(db *sql.DB) (*DB, error) {
	if _, err := db.Exec(dbSchema); err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

// Channels returns the saved channels with their links.
func (d *DB) Channels() ([]Channel, error) {
	rows, err := d.db.Query(`SELECT id, parent, name, description, position FROM mumble_channels`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[uint32]*Channel{}
	var order []uint32
	for rows.Next() {
		c := &Channel{Links: map[uint32]bool{}}
		if err := rows.Scan(&c.ID, &c.Parent, &c.Name, &c.Description, &c.Position); err != nil {
			return nil, err
		}
		byID[c.ID] = c
		order = append(order, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	links, err := d.db.Query(`SELECT a, b FROM mumble_channel_links`)
	if err != nil {
		return nil, err
	}
	defer links.Close()
	for links.Next() {
		var a, b uint32
		if err := links.Scan(&a, &b); err != nil {
			return nil, err
		}
		if byID[a] != nil {
			byID[a].Links[b] = true
		}
	}
	out := make([]Channel, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, links.Err()
}

// SaveChannel writes a channel and its links (each link once, a < b).
func (d *DB) SaveChannel(c *Channel) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO mumble_channels (id, parent, name, description, position) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET parent = excluded.parent, name = excluded.name,
		description = excluded.description, position = excluded.position`,
		c.ID, c.Parent, c.Name, c.Description, c.Position); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM mumble_channel_links WHERE a = ? OR b = ?`, c.ID, c.ID); err != nil {
		return err
	}
	for l := range c.Links {
		a, b := min(c.ID, l), max(c.ID, l)
		// The other channel may not be saved yet (it is saved next).
		if _, err := tx.Exec(`INSERT OR IGNORE INTO mumble_channel_links (a, b)
			SELECT ?, ? WHERE EXISTS (SELECT 1 FROM mumble_channels WHERE id = ?)`, a, b, l); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteChannels removes channels (a subtree) at once.
func (d *DB) DeleteChannels(ids []uint32) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM mumble_channel_links WHERE a = ? OR b = ?`, id, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM mumble_channels WHERE id = ? AND id != 0`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// NextChannel is the highest channel id given out, so ids of removed and
// temporary channels are not reused.
func (d *DB) NextChannel() (uint32, error) {
	var n uint32
	err := d.db.QueryRow(`SELECT value FROM mumble_meta WHERE key = 'last_channel_id'`).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}

func (d *DB) SaveNextChannel(n uint32) error {
	_, err := d.db.Exec(`INSERT INTO mumble_meta (key, value) VALUES ('last_channel_id', ?)
		ON CONFLICT (key) DO UPDATE SET value = max(value, excluded.value)`, n)
	return err
}

const userColumns = `id, name, COALESCE(cert_hash, ''), password_hash, last_channel`

func (d *DB) userWhere(where string, arg any) (*Registered, error) {
	var r Registered
	err := d.db.QueryRow(`SELECT `+userColumns+` FROM mumble_users WHERE `+where, arg).
		Scan(&r.ID, &r.Name, &r.CertHash, &r.PasswordHash, &r.LastChannel)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UserByName finds a registered user by name (compared as names.Key).
func (d *DB) UserByName(name string) (*Registered, error) {
	return d.userWhere(`name_key = ?`, names.Key(name))
}

func (d *DB) UserByHash(hash string) (*Registered, error) {
	if hash == "" {
		return nil, nil
	}
	return d.userWhere(`cert_hash = ?`, hash)
}

// RegisterUser adds a user with a certificate and returns its id.
func (d *DB) RegisterUser(name, certHash string) (uint32, error) {
	res, err := d.db.Exec(`INSERT INTO mumble_users (name, name_key, cert_hash) VALUES (?, ?, NULLIF(?, ''))`,
		name, names.Key(name), certHash)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint32(id), err
}

func (d *DB) SetPassword(id uint32, passwordHash string) error {
	_, err := d.db.Exec(`UPDATE mumble_users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	return err
}

func (d *DB) SetLastChannel(id, channel uint32) error {
	_, err := d.db.Exec(`UPDATE mumble_users SET last_channel = ? WHERE id = ?`, channel, id)
	return err
}

// UserNames returns the names of the registered users among ids.
func (d *DB) UserNames(ids []uint32) (map[uint32]string, error) {
	out := map[uint32]string{}
	for _, id := range ids {
		var name string
		err := d.db.QueryRow(`SELECT name FROM mumble_users WHERE id = ?`, id).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, nil
}
