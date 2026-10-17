package store

import (
	"database/sql"
	"errors"
)

const statusTable = `CREATE TABLE IF NOT EXISTS user_status (
	user_id INTEGER PRIMARY KEY, -- the Mumble user id
	status  TEXT NOT NULL,       -- "", "idle" or "dnd"
	emoji   TEXT NOT NULL,
	text    TEXT NOT NULL,
	until   INTEGER NOT NULL     -- Unix milliseconds; 0: no end
)`

// UserStatus is the status a registered user chose, and a custom status
// (emoji and text, until a time). The zero value: online, no custom status.
type UserStatus struct {
	Status string
	Emoji  string
	Text   string
	Until  int64
}

// SetUserStatus keeps a user's status (the zero value removes it).
func (s *Store) SetUserStatus(userID uint32, st UserStatus) error {
	if st == (UserStatus{}) {
		_, err := s.db.Exec(`DELETE FROM user_status WHERE user_id = ?`, userID)
		return err
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO user_status (user_id, status, emoji, text, until) VALUES (?, ?, ?, ?, ?)`,
		userID, st.Status, st.Emoji, st.Text, st.Until)
	return err
}

// UserStatus is a user's kept status (the zero value if there is none).
func (s *Store) UserStatus(userID uint32) (UserStatus, error) {
	var st UserStatus
	err := s.db.QueryRow(`SELECT status, emoji, text, until FROM user_status WHERE user_id = ?`, userID).
		Scan(&st.Status, &st.Emoji, &st.Text, &st.Until)
	if errors.Is(err, sql.ErrNoRows) {
		return UserStatus{}, nil
	}
	return st, err
}
