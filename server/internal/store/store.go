// Package store keeps text channels and their messages in SQLite.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"kallisti.chat/server/internal/names"
)

type Channel struct {
	ID       int64
	Name     string
	Position int
	// Mumble is the channel that mirrors it for Mumble clients (0: none).
	Mumble uint32
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

// DB is the database, for other parts that keep their tables in the same
// file (the built-in Mumble server).
func (s *Store) DB() *sql.DB { return s.db }

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
CREATE TABLE IF NOT EXISTS accounts (
	id            INTEGER PRIMARY KEY,
	name          TEXT NOT NULL,
	name_key      TEXT NOT NULL UNIQUE, -- see names.Key
	password_hash TEXT NOT NULL,
	cert_pem      BLOB NOT NULL,
	key_pem       BLOB NOT NULL,
	created_at    INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
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
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('channels') WHERE name = 'voice_channel'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		// The Mumble channel whose chat this hidden channel is.
		if _, err := db.Exec(`ALTER TABLE channels ADD COLUMN voice_channel INTEGER`); err != nil {
			return err
		}
		if _, err := db.Exec(`CREATE UNIQUE INDEX channels_voice ON channels (voice_channel) WHERE voice_channel IS NOT NULL`); err != nil {
			return err
		}
	}
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('channels') WHERE name = 'mumble_channel'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		// The Mumble channel that mirrors a text channel.
		if _, err := db.Exec(`ALTER TABLE channels ADD COLUMN mumble_channel INTEGER`); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

// VoiceChat is the hidden channel that holds a voice channel's chat (the
// Mumble channel id; the built-in server never reuses them), made on first
// use.
func (s *Store) VoiceChat(voice uint32) (int64, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM channels WHERE voice_channel = ?`, voice).Scan(&id)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return id, err
	}
	// The name only has to be unique; text channel names cannot have ":".
	err = s.db.QueryRow(`INSERT INTO channels (name, position, hidden, voice_channel) VALUES (?, 0, 1, ?)
		ON CONFLICT (voice_channel) WHERE voice_channel IS NOT NULL DO UPDATE SET hidden = 1 RETURNING id`,
		fmt.Sprintf("voice:%d", voice), voice).Scan(&id)
	return id, err
}

// VoiceOf is the voice channel whose chat a channel is (ok false for text
// channels).
func (s *Store) VoiceOf(channel int64) (voice uint32, ok bool, err error) {
	var v sql.NullInt64
	err = s.db.QueryRow(`SELECT voice_channel FROM channels WHERE id = ?`, channel).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return uint32(v.Int64), v.Valid, err
}

// EnsureChannels makes the configured channels visible, in this order.
// Channels that are no longer configured are hidden, not deleted, so their
// history comes back if they are configured again.
func (s *Store) EnsureChannels(names []string) error {
	for _, n := range names {
		if strings.Contains(n, ":") {
			return ErrInvalidName
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE channels SET hidden = 1 WHERE voice_channel IS NULL`); err != nil {
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

// SeedChannels makes the first text channels of a new server (later they
// are managed in the client).
func (s *Store) SeedChannels(names []string) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM channels WHERE voice_channel IS NULL`).Scan(&n); err != nil || n > 0 {
		return err
	}
	return s.EnsureChannels(names)
}

// CreateChannel adds a text channel at the end. A deleted channel of that
// name comes back, with its history.
func (s *Store) CreateChannel(name string) (Channel, error) {
	if strings.Contains(name, ":") {
		return Channel{}, ErrInvalidName
	}
	c := Channel{Name: name}
	err := s.db.QueryRow(`INSERT INTO channels (name, position)
		VALUES (?, (SELECT COALESCE(MAX(position), -1) + 1 FROM channels WHERE hidden = 0))
		ON CONFLICT (name) DO UPDATE SET hidden = 0, position = excluded.position
		WHERE hidden = 1 AND voice_channel IS NULL
		RETURNING id, position`, name).Scan(&c.ID, &c.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, ErrNameTaken // a visible channel has the name
	}
	return c, err
}

// textChannel runs a change on a text channel (not a voice channel chat).
func (s *Store) textChannel(query string, args ...any) error {
	res, err := s.db.Exec(query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrNameTaken
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RenameChannel(id int64, name string) error {
	if strings.Contains(name, ":") {
		return ErrInvalidName
	}
	return s.textChannel(`UPDATE channels SET name = ? WHERE id = ? AND voice_channel IS NULL`, name, id)
}

// HideChannel removes a text channel from the list; its history stays.
func (s *Store) HideChannel(id int64) error {
	return s.textChannel(`UPDATE channels SET hidden = 1 WHERE id = ? AND voice_channel IS NULL`, id)
}

// SetPositions puts the given text channels in this order.
func (s *Store) SetPositions(ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE channels SET position = ? WHERE id = ? AND voice_channel IS NULL`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetMumbleChannel remembers the Mumble channel that mirrors a text channel.
func (s *Store) SetMumbleChannel(id int64, mumble uint32) error {
	_, err := s.db.Exec(`UPDATE channels SET mumble_channel = ? WHERE id = ? AND voice_channel IS NULL`, mumble, id)
	return err
}

// Channels returns the visible channels in order.
func (s *Store) Channels() ([]Channel, error) {
	rows, err := s.db.Query(`SELECT id, name, position, COALESCE(mumble_channel, 0) FROM channels WHERE hidden = 0 ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.Name, &c.Position, &c.Mumble); err != nil {
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

// UploadedBytesSince sums the sizes of an author's uploads since a time
// (for the daily quota). Deleted unused uploads do not count.
func (s *Store) UploadedBytesSince(author string, since time.Time) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT coalesce(sum(size), 0) FROM uploads WHERE author = ? AND created_at >= ?`,
		author, since.UnixMilli()).Scan(&n)
	return n, err
}

var (
	ErrNotFound  = errors.New("not found")
	ErrNameTaken = errors.New("name taken")
	// Text channel names cannot have ":" (voice channel chats use it).
	ErrInvalidName = errors.New("invalid channel name")
)

// Account is a web user. Names compare by names.Key (case, NFC, spaces).
type Account struct {
	ID           int64
	Name         string
	PasswordHash string
	CertPEM      []byte
	KeyPEM       []byte
	CreatedAt    time.Time
}

func (s *Store) CreateAccount(a Account, at time.Time) (Account, error) {
	res, err := s.db.Exec(`INSERT INTO accounts (name, name_key, password_hash, cert_pem, key_pem, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		a.Name, names.Key(a.Name), a.PasswordHash, a.CertPEM, a.KeyPEM, at.UnixMilli())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Account{}, ErrNameTaken
		}
		return Account{}, err
	}
	a.ID, err = res.LastInsertId()
	a.CreatedAt = at
	return a, err
}

const accountColumns = `a.id, a.name, a.password_hash, a.cert_pem, a.key_pem, a.created_at`

func scanAccount(row *sql.Row) (Account, error) {
	var a Account
	var at int64
	err := row.Scan(&a.ID, &a.Name, &a.PasswordHash, &a.CertPEM, &a.KeyPEM, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	a.CreatedAt = time.UnixMilli(at).UTC()
	return a, err
}

func (s *Store) AccountByName(name string) (Account, error) {
	return scanAccount(s.db.QueryRow(`SELECT `+accountColumns+` FROM accounts a WHERE a.name_key = ?`, names.Key(name)))
}

// Sessions are stored by the hash of their token: a leaked database does
// not give working session cookies.
func (s *Store) CreateSession(tokenHash string, accountID int64, expires time.Time) error {
	_, err := s.db.Exec(`INSERT INTO sessions (token_hash, account_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash, accountID, expires.UnixMilli())
	return err
}

func (s *Store) AccountBySession(tokenHash string, now time.Time) (Account, error) {
	return scanAccount(s.db.QueryRow(`SELECT `+accountColumns+` FROM sessions x JOIN accounts a ON a.id = x.account_id
		WHERE x.token_hash = ? AND x.expires_at > ?`, tokenHash, now.UnixMilli()))
}

func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteExpiredSessions(now time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) SetPassword(accountID int64, hash string) error {
	_, err := s.db.Exec(`UPDATE accounts SET password_hash = ? WHERE id = ?`, hash, accountID)
	return err
}

// DeleteSessionsExcept ends all sessions of an account but one ("" ends all).
func (s *Store) DeleteSessionsExcept(accountID int64, keepTokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE account_id = ? AND token_hash != ?`, accountID, keepTokenHash)
	return err
}
