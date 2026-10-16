package store

import (
	"database/sql"
	"errors"
	"time"
)

// MaxPins is the most pinned messages per channel (as Discord).
const MaxPins = 50

var ErrTooManyPins = errors.New("too many pinned messages in this channel")

// SetPinned pins a message (at) or unpins it (zero at); changed is false
// when it already was so.
func (s *Store) SetPinned(id int64, at time.Time) (m Message, changed bool, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Message{}, false, err
	}
	defer tx.Rollback()
	var channel int64
	var pinned sql.NullInt64
	err = tx.QueryRow(`SELECT channel_id, pinned_at FROM messages WHERE id = ?`, id).Scan(&channel, &pinned)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, false, ErrNotFound
	}
	if err != nil {
		return Message{}, false, err
	}
	if pin := !at.IsZero(); pin != pinned.Valid {
		if pin {
			var n int
			if err := tx.QueryRow(`SELECT count(*) FROM messages WHERE channel_id = ? AND pinned_at IS NOT NULL`, channel).Scan(&n); err != nil {
				return Message{}, false, err
			}
			if n >= MaxPins {
				return Message{}, false, ErrTooManyPins
			}
		}
		value := sql.NullInt64{Int64: at.UnixMilli(), Valid: pin}
		if _, err := tx.Exec(`UPDATE messages SET pinned_at = ? WHERE id = ?`, value, id); err != nil {
			return Message{}, false, err
		}
		changed = true
	}
	if err := tx.Commit(); err != nil {
		return Message{}, false, err
	}
	m, err = s.MessageByID(id)
	return m, changed, err
}

// Pinned returns a channel's pinned messages, the newest pin first.
func (s *Store) Pinned(channelID int64) ([]Message, error) {
	rows, err := s.db.Query(messageSelect+`WHERE m.channel_id = ? AND m.pinned_at IS NOT NULL ORDER BY m.pinned_at DESC, m.id DESC LIMIT ?`,
		channelID, MaxPins)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, s.withReactions(out)
}
