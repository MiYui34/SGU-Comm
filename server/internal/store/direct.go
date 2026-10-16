package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Direct chats: one hidden channel per pair of registered users (by user
// id), holding their direct messages.

// DirectChat finds or makes the chat of two users.
func (s *Store) DirectChat(a, b uint32) (int64, error) {
	if a == b {
		return 0, errors.New("a direct chat needs two users")
	}
	low, high := min(a, b), max(a, b)
	var id int64
	// The name only has to be unique; text channel names cannot have ":".
	err := s.db.QueryRow(`INSERT INTO channels (name, position, hidden, dm_low, dm_high) VALUES (?, 0, 1, ?, ?)
		ON CONFLICT (dm_low, dm_high) WHERE dm_low IS NOT NULL DO UPDATE SET hidden = 1 RETURNING id`,
		fmt.Sprintf("dm:%d:%d", low, high), low, high).Scan(&id)
	return id, err
}

// DirectPeers are the two users of a direct chat (ok false: not one).
func (s *Store) DirectPeers(channel int64) (a, b uint32, ok bool, err error) {
	var low, high sql.NullInt64
	err = s.db.QueryRow(`SELECT dm_low, dm_high FROM channels WHERE id = ?`, channel).Scan(&low, &high)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	return uint32(low.Int64), uint32(high.Int64), err == nil && low.Valid, err
}

// DirectChats are a user's direct chats with messages: chat -> the other
// user.
func (s *Store) DirectChats(user uint32) (map[int64]uint32, error) {
	rows, err := s.db.Query(`SELECT id, dm_low, dm_high FROM channels c WHERE (dm_low = ? OR dm_high = ?)
		AND EXISTS (SELECT 1 FROM messages WHERE channel_id = c.id)`, user, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]uint32{}
	for rows.Next() {
		var id int64
		var low, high uint32
		if err := rows.Scan(&id, &low, &high); err != nil {
			return nil, err
		}
		out[id] = low + high - user
	}
	return out, rows.Err()
}
