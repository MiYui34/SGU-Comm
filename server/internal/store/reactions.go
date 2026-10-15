package store

import (
	"strings"
	"time"

	"kallisti.chat/server/internal/names"
)

// At most this many different emoji per message.
const maxReactionKinds = 20

// Reaction is one emoji on a message and who chose it (first first).
type Reaction struct {
	Emoji string
	Users []string
}

// SetReaction adds (on) or removes a user's reaction to a message.
func (s *Store) SetReaction(messageID int64, emoji, user string, on bool, at time.Time) error {
	key := names.Key(user)
	if !on {
		_, err := s.db.Exec(`DELETE FROM reactions WHERE message_id = ? AND emoji = ? AND user_key = ?`, messageID, emoji, key)
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists, known, kinds int
	err = tx.QueryRow(`SELECT (SELECT count(*) FROM messages WHERE id = ?),
		(SELECT count(*) FROM reactions WHERE message_id = ? AND emoji = ?),
		(SELECT count(DISTINCT emoji) FROM reactions WHERE message_id = ?)`,
		messageID, messageID, emoji, messageID).Scan(&exists, &known, &kinds)
	switch {
	case err != nil:
		return err
	case exists == 0:
		return ErrNotFound
	case known == 0 && kinds >= maxReactionKinds:
		return ErrTooManyReactions
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO reactions (message_id, emoji, user_key, user, created_at) VALUES (?, ?, ?, ?, ?)`,
		messageID, emoji, key, user, at.UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}

// withReactions fills in the reactions of messages, in the order they
// were first given.
func (s *Store) withReactions(msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	at := map[int64]int{}
	args := make([]any, len(msgs))
	for i, m := range msgs {
		at[m.ID], args[i] = i, m.ID
	}
	rows, err := s.db.Query(`SELECT message_id, emoji, user FROM reactions
		WHERE message_id IN (?`+strings.Repeat(",?", len(msgs)-1)+`)
		ORDER BY message_id, (SELECT min(created_at) FROM reactions r WHERE r.message_id = reactions.message_id AND r.emoji = reactions.emoji),
			emoji, created_at, rowid`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var emoji, user string
		if err := rows.Scan(&id, &emoji, &user); err != nil {
			return err
		}
		m := &msgs[at[id]]
		if n := len(m.Reactions); n > 0 && m.Reactions[n-1].Emoji == emoji {
			m.Reactions[n-1].Users = append(m.Reactions[n-1].Users, user)
		} else {
			m.Reactions = append(m.Reactions, Reaction{Emoji: emoji, Users: []string{user}})
		}
	}
	return rows.Err()
}
