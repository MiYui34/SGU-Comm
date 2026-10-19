package store

import "time"

// Stream keys (streaming from OBS): one per account, stored by the hash of
// the key, like sessions.
const streamKeysTable = `CREATE TABLE IF NOT EXISTS stream_keys (
	account_id INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
	key_hash   TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL
)`

// SetStreamKey gives an account a new key (the old one stops working).
func (s *Store) SetStreamKey(accountID int64, keyHash string, at time.Time) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO stream_keys (account_id, key_hash, created_at) VALUES (?, ?, ?)`,
		accountID, keyHash, at.UnixMilli())
	return err
}

func (s *Store) DeleteStreamKey(accountID int64) error {
	_, err := s.db.Exec(`DELETE FROM stream_keys WHERE account_id = ?`, accountID)
	return err
}

func (s *Store) HasStreamKey(accountID int64) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT count(*) FROM stream_keys WHERE account_id = ?`, accountID).Scan(&n)
	return n > 0, err
}

func (s *Store) AccountByStreamKey(keyHash string) (Account, error) {
	return scanAccount(s.db.QueryRow(`SELECT `+accountColumns+` FROM stream_keys k JOIN accounts a ON a.id = k.account_id
		WHERE k.key_hash = ?`, keyHash))
}
