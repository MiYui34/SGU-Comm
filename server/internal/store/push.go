package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Web Push: a browser's subscription belongs to the login session that
// made it, so it ends with the session (log out, expiry, password change).
const pushTables = `CREATE TABLE IF NOT EXISTS push_subscriptions (
	endpoint   TEXT PRIMARY KEY,
	session    TEXT NOT NULL REFERENCES sessions(token_hash) ON DELETE CASCADE,
	p256dh     TEXT NOT NULL,
	auth       TEXT NOT NULL,
	quiet      TEXT NOT NULL, -- JSON: text channels set to "nothing" there
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS push_by_session ON push_subscriptions(session);
CREATE TABLE IF NOT EXISTS push_vapid (
	id          INTEGER PRIMARY KEY CHECK (id = 1),
	public_key  TEXT NOT NULL,
	private_key TEXT NOT NULL
)`

// Subscriptions per account, at most: a new one replaces the oldest.
const maxPushSubscriptions = 10

// PushSubscription is a browser's push endpoint and keys (RFC 8291), and
// the text channels it wants no pushes for.
type PushSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
	Quiet    []int64
}

// SetPushSubscription keeps a subscription for a session (by its endpoint:
// the same browser again replaces it, also for another account).
func (s *Store) SetPushSubscription(session string, p PushSubscription, at time.Time) error {
	quiet, err := json.Marshal(append([]int64{}, p.Quiet...))
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR REPLACE INTO push_subscriptions (endpoint, session, p256dh, auth, quiet, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		p.Endpoint, session, p.P256dh, p.Auth, string(quiet), at.UnixMilli()); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM push_subscriptions WHERE endpoint IN (
		SELECT p.endpoint FROM push_subscriptions p JOIN sessions x ON x.token_hash = p.session
		WHERE x.account_id = (SELECT account_id FROM sessions WHERE token_hash = ?)
		ORDER BY p.created_at DESC LIMIT -1 OFFSET ?)`, session, maxPushSubscriptions); err != nil {
		return err
	}
	return tx.Commit()
}

// DeletePushSubscription removes an account's subscription.
func (s *Store) DeletePushSubscription(accountID int64, endpoint string) error {
	_, err := s.db.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?
		AND session IN (SELECT token_hash FROM sessions WHERE account_id = ?)`, endpoint, accountID)
	return err
}

// DropPushEndpoint removes a subscription the push service no longer has.
func (s *Store) DropPushEndpoint(endpoint string) error {
	_, err := s.db.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}

// pushOf joins subscriptions to valid sessions of permanent accounts.
const pushOf = ` FROM push_subscriptions p JOIN sessions x ON x.token_hash = p.session
	JOIN accounts a ON a.id = x.account_id WHERE x.expires_at > ? AND a.temporary = 0`

// PushSubscriptions are an account's subscriptions (by names.Key).
func (s *Store) PushSubscriptions(nameKey string, now time.Time) ([]PushSubscription, error) {
	rows, err := s.db.Query(`SELECT p.endpoint, p.p256dh, p.auth, p.quiet`+pushOf+` AND a.name_key = ?`, now.UnixMilli(), nameKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PushSubscription
	for rows.Next() {
		var p PushSubscription
		var quiet string
		if err := rows.Scan(&p.Endpoint, &p.P256dh, &p.Auth, &quiet); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(quiet), &p.Quiet); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PushNames are the names of the accounts with subscriptions.
func (s *Store) PushNames(now time.Time) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT a.name`+pushOf+` ORDER BY a.name`, now.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// VAPIDKeys are the server's push keys (RFC 8292), made with generate on
// the first call.
func (s *Store) VAPIDKeys(generate func() (public, private string, err error)) (string, string, error) {
	var public, private string
	err := s.db.QueryRow(`SELECT public_key, private_key FROM push_vapid`).Scan(&public, &private)
	if !errors.Is(err, sql.ErrNoRows) {
		return public, private, err
	}
	if public, private, err = generate(); err != nil {
		return "", "", err
	}
	_, err = s.db.Exec(`INSERT INTO push_vapid (id, public_key, private_key) VALUES (1, ?, ?)`, public, private)
	return public, private, err
}
