package store

import (
	"strings"
	"time"
)

// At most this many different emoji per message.
const maxReactionKinds = 20

// Reaction is one emoji on a message and who chose it (first first): user
// ids and the names to show.
type Reaction struct {
	Emoji string
	Users []string
	IDs   []uint32
}

// SetReaction adds (on) or removes a registered user's reaction to a
// message; changed is false when it was already so.
func (s *Store) SetReaction(messageID int64, emoji string, userID uint32, user string, on bool, at time.Time) (changed bool, err error) {
	if !on {
		res, err := s.db.Exec(`DELETE FROM reactions WHERE message_id = ? AND emoji = ? AND user_id = ?`, messageID, emoji, userID)
		if err != nil {
			return false, err
		}
		n, err := res.RowsAffected()
		return n > 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var exists, known, kinds int
	err = tx.QueryRow(`SELECT (SELECT count(*) FROM messages WHERE id = ?),
		(SELECT count(*) FROM reactions WHERE message_id = ? AND emoji = ?),
		(SELECT count(DISTINCT emoji) FROM reactions WHERE message_id = ?)`,
		messageID, messageID, emoji, messageID).Scan(&exists, &known, &kinds)
	switch {
	case err != nil:
		return false, err
	case exists == 0:
		return false, ErrNotFound
	case known == 0 && kinds >= maxReactionKinds:
		return false, ErrTooManyReactions
	}
	res, err := tx.Exec(`INSERT OR IGNORE INTO reactions (message_id, emoji, user_id, user, created_at) VALUES (?, ?, ?, ?, ?)`,
		messageID, emoji, userID, user, at.UnixMilli())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, tx.Commit()
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
	rows, err := s.db.Query(`SELECT message_id, emoji, user_id, user FROM reactions
		WHERE message_id IN (?`+strings.Repeat(",?", len(msgs)-1)+`)
		ORDER BY message_id, (SELECT min(created_at) FROM reactions r WHERE r.message_id = reactions.message_id AND r.emoji = reactions.emoji),
			emoji, created_at, rowid`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var userID uint32
		var emoji, user string
		if err := rows.Scan(&id, &emoji, &userID, &user); err != nil {
			return err
		}
		m := &msgs[at[id]]
		if n := len(m.Reactions); n > 0 && m.Reactions[n-1].Emoji == emoji {
			r := &m.Reactions[n-1]
			r.Users, r.IDs = append(r.Users, user), append(r.IDs, userID)
		} else {
			m.Reactions = append(m.Reactions, Reaction{Emoji: emoji, Users: []string{user}, IDs: []uint32{userID}})
		}
	}
	return rows.Err()
}
