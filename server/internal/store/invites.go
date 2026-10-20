package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Invite is a link to a private Mumble channel (see the sidecar). Only the
// token's hash is kept. Zero ExpiresAt: never; zero MaxUses: unlimited.
type Invite struct {
	ID        int64
	Channel   uint32
	CreatorID uint32
	Creator   string
	CreatedAt time.Time
	ExpiresAt time.Time
	MaxUses   int
	Uses      int
}

const invitesTable = `CREATE TABLE IF NOT EXISTS invites (
	id         INTEGER PRIMARY KEY,
	token_hash TEXT NOT NULL UNIQUE,
	channel    INTEGER NOT NULL, -- the Mumble channel
	creator_id INTEGER NOT NULL,
	creator    TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL, -- Unix milliseconds; 0: never
	max_uses   INTEGER NOT NULL, -- 0: unlimited
	uses       INTEGER NOT NULL DEFAULT 0
)`

// validInvite: not expired (at ?) and not used up.
const validInvite = `(expires_at = 0 OR expires_at > ?) AND (max_uses = 0 OR uses < max_uses)`

const inviteSelect = `SELECT id, channel, creator_id, creator, created_at, expires_at, max_uses, uses FROM invites `

func ms(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func scanInvite(row interface{ Scan(...any) error }) (Invite, error) {
	var i Invite
	var created, expires int64
	if err := row.Scan(&i.ID, &i.Channel, &i.CreatorID, &i.Creator, &created, &expires, &i.MaxUses, &i.Uses); err != nil {
		return Invite{}, err
	}
	i.CreatedAt = time.UnixMilli(created).UTC()
	if expires != 0 {
		i.ExpiresAt = time.UnixMilli(expires).UTC()
	}
	return i, nil
}

// AddInvite keeps an invite by its token's hash.
func (s *Store) AddInvite(hash string, i Invite) (Invite, error) {
	err := s.db.QueryRow(`INSERT INTO invites (token_hash, channel, creator_id, creator, created_at, expires_at, max_uses)
		VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		hash, i.Channel, i.CreatorID, i.Creator, i.CreatedAt.UnixMilli(), ms(i.ExpiresAt), i.MaxUses).Scan(&i.ID)
	return i, err
}

// Invites lists a channel's valid invites, the oldest first.
func (s *Store) Invites(channel uint32, now time.Time) ([]Invite, error) {
	rows, err := s.db.Query(inviteSelect+`WHERE channel = ? AND `+validInvite+` ORDER BY id`, channel, now.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invite{}
	for rows.Next() {
		i, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// InviteByHash is a valid invite (ErrNotFound: unknown, expired or used up).
func (s *Store) InviteByHash(hash string, now time.Time) (Invite, error) {
	i, err := scanInvite(s.db.QueryRow(inviteSelect+`WHERE token_hash = ? AND `+validInvite, hash, now.UnixMilli()))
	if errors.Is(err, sql.ErrNoRows) {
		return Invite{}, ErrNotFound
	}
	return i, err
}

// UseInvite counts a use, if the invite is still valid (else ErrNotFound).
func (s *Store) UseInvite(id int64, now time.Time) error {
	res, err := s.db.Exec(`UPDATE invites SET uses = uses + 1 WHERE id = ? AND `+validInvite, id, now.UnixMilli())
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// InviteChannel says whose channel an invite is (valid or not).
func (s *Store) InviteChannel(id int64) (channel uint32, err error) {
	err = s.db.QueryRow(`SELECT channel FROM invites WHERE id = ?`, id).Scan(&channel)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return channel, err
}

// DeleteInvite deletes an invite and says whose channel it was.
func (s *Store) DeleteInvite(id int64) (channel uint32, err error) {
	err = s.db.QueryRow(`DELETE FROM invites WHERE id = ? RETURNING channel`, id).Scan(&channel)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return channel, err
}

// PruneInvites deletes invites that are not valid any more, and those of
// channels not in private.
func (s *Store) PruneInvites(private []uint32, now time.Time) error {
	ids, err := json.Marshal(append([]uint32{}, private...))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM invites WHERE channel NOT IN (SELECT value FROM json_each(?)) OR NOT (`+validInvite+`)`,
		string(ids), now.UnixMilli())
	return err
}
