package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// Personal emoji: each registered user has a list of names for emoji
// images. An image is stored once (by content hash) and shared; messages
// refer to it by id, so it stays when nobody has it any more. A removed
// image keeps its row without data: its hash is blocked and its name
// stays for reactions.

// MaxUserEmoji is the most personal emoji a user has, and the most new
// images a user adds in a day.
const MaxUserEmoji = 50

var (
	ErrEmojiOwned      = errors.New("the user has this image already")
	ErrEmojiBlocked    = errors.New("the image was removed by a moderator")
	ErrTooManyNewEmoji = errors.New("too many new emoji images today")
)

const userEmojiTables = `CREATE TABLE IF NOT EXISTS emoji_images (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	hash       TEXT NOT NULL UNIQUE,
	name       TEXT NOT NULL,
	type       TEXT NOT NULL,
	data       BLOB,
	uploader   INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS emoji_images_uploader ON emoji_images (uploader, created_at);
CREATE TABLE IF NOT EXISTS user_emoji (
	user_id    INTEGER NOT NULL,
	name       TEXT NOT NULL,
	image_id   INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (user_id, name),
	UNIQUE (user_id, image_id)
)`

// UserEmoji is one of a user's emoji.
type UserEmoji struct {
	Name    string
	ImageID int64
}

// EmojiImage is a personal emoji image; Name: the first name it had.
type EmojiImage struct {
	ID        int64
	Name      string
	Type      string
	Data      []byte
	CreatedAt time.Time
}

// AddUserEmoji adds an emoji with this image (the caller checks name and
// image); a known image is used again.
func (s *Store) AddUserEmoji(user uint32, name, ctype string, data []byte, at time.Time) (UserEmoji, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	return s.addUserEmoji(user, name, at, func(tx *sql.Tx) (int64, error) {
		var id int64
		var removed bool
		err := tx.QueryRow(`SELECT id, data IS NULL FROM emoji_images WHERE hash = ?`, hash).Scan(&id, &removed)
		switch {
		case err == nil && removed:
			return 0, ErrEmojiBlocked
		case err == nil:
			return id, nil
		case !errors.Is(err, sql.ErrNoRows):
			return 0, err
		}
		var n int
		since := at.Add(-24 * time.Hour).UnixMilli()
		if err := tx.QueryRow(`SELECT count(*) FROM emoji_images WHERE uploader = ? AND created_at > ?`, user, since).Scan(&n); err != nil {
			return 0, err
		}
		if n >= MaxUserEmoji {
			return 0, ErrTooManyNewEmoji
		}
		err = tx.QueryRow(`INSERT INTO emoji_images (hash, name, type, data, uploader, created_at) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
			hash, name, ctype, data, user, at.UnixMilli()).Scan(&id)
		return id, err
	})
}

// AddUserEmojiImage adds an emoji with an image that is there (not
// removed).
func (s *Store) AddUserEmojiImage(user uint32, name string, image int64, at time.Time) (UserEmoji, error) {
	return s.addUserEmoji(user, name, at, func(tx *sql.Tx) (int64, error) {
		var n int
		if err := tx.QueryRow(`SELECT count(*) FROM emoji_images WHERE id = ? AND data IS NOT NULL`, image).Scan(&n); err != nil {
			return 0, err
		} else if n == 0 {
			return 0, ErrNotFound
		}
		return image, nil
	})
}

// addUserEmoji checks the user's limit and names, then adds the image
// that imageOf finds or stores.
func (s *Store) addUserEmoji(user uint32, name string, at time.Time, imageOf func(*sql.Tx) (int64, error)) (UserEmoji, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return UserEmoji{}, err
	}
	defer tx.Rollback()
	var count, taken int
	err = tx.QueryRow(`SELECT count(*), count(CASE WHEN name = ? THEN 1 END) FROM user_emoji WHERE user_id = ?`, name, user).Scan(&count, &taken)
	switch {
	case err != nil:
		return UserEmoji{}, err
	case taken > 0:
		return UserEmoji{}, ErrNameTaken
	case count >= MaxUserEmoji:
		return UserEmoji{}, ErrTooManyEmoji
	}
	image, err := imageOf(tx)
	if err != nil {
		return UserEmoji{}, err
	}
	if _, err := tx.Exec(`INSERT INTO user_emoji (user_id, name, image_id, created_at) VALUES (?, ?, ?, ?)`, user, name, image, at.UnixMilli()); err != nil {
		if nameTaken(err) == ErrNameTaken { // the name is free, so the image is the user's already
			return UserEmoji{}, ErrEmojiOwned
		}
		return UserEmoji{}, err
	}
	return UserEmoji{Name: name, ImageID: image}, tx.Commit()
}

// UserEmojis lists a user's emoji by name.
func (s *Store) UserEmojis(user uint32) ([]UserEmoji, error) {
	rows, err := s.db.Query(`SELECT name, image_id FROM user_emoji WHERE user_id = ? ORDER BY name`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserEmoji
	for rows.Next() {
		var e UserEmoji
		if err := rows.Scan(&e.Name, &e.ImageID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) RenameUserEmoji(user uint32, name, to string) error {
	return rowChanged(s.db.Exec(`UPDATE user_emoji SET name = ? WHERE user_id = ? AND name = ?`, to, user, name))
}

func (s *Store) DeleteUserEmoji(user uint32, name string) error {
	return rowChanged(s.db.Exec(`DELETE FROM user_emoji WHERE user_id = ? AND name = ?`, user, name))
}

// EmojiImage is an image that was not removed.
func (s *Store) EmojiImage(id int64) (EmojiImage, error) {
	e := EmojiImage{ID: id}
	var at int64
	err := s.db.QueryRow(`SELECT name, type, data, created_at FROM emoji_images WHERE id = ? AND data IS NOT NULL`, id).
		Scan(&e.Name, &e.Type, &e.Data, &at)
	e.CreatedAt = time.UnixMilli(at).UTC()
	return e, notFound(err)
}

// EmojiImageName is an image's first name, also when it was removed.
func (s *Store) EmojiImageName(id int64) (name string, removed bool, err error) {
	err = s.db.QueryRow(`SELECT name, data IS NULL FROM emoji_images WHERE id = ?`, id).Scan(&name, &removed)
	return name, removed, notFound(err)
}

// RemoveEmojiImage removes an image everywhere (moderators): its data
// goes, nobody has it any more, and its hash is blocked. owners: who had
// it.
func (s *Store) RemoveEmojiImage(id int64) (owners []uint32, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := rowChanged(tx.Exec(`UPDATE emoji_images SET data = NULL WHERE id = ? AND data IS NOT NULL`, id)); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`DELETE FROM user_emoji WHERE image_id = ? RETURNING user_id`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var u uint32
		if err := rows.Scan(&u); err != nil {
			rows.Close()
			return nil, err
		}
		owners = append(owners, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return owners, tx.Commit()
}

// RemovedEmojiImages lists the removed images' ids.
func (s *Store) RemovedEmojiImages() ([]int64, error) {
	rows, err := s.db.Query(`SELECT id FROM emoji_images WHERE data IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// notFound: ErrNotFound for no row.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
