package mumbled

import (
	"database/sql"
	"errors"
	"net/netip"
	"time"

	"kallisti.chat/server/internal/names"
)

// DB stores the Mumble server's data in the Kallisti server's SQLite file.
type DB struct{ db *sql.DB }

const dbSchema = `
CREATE TABLE IF NOT EXISTS mumble_channels (
	id          INTEGER PRIMARY KEY,
	parent      INTEGER NOT NULL,
	name        TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	position    INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS mumble_acl (
	channel_id INTEGER NOT NULL,
	priority   INTEGER NOT NULL,
	apply_here INTEGER NOT NULL,
	apply_subs INTEGER NOT NULL,
	user_id    INTEGER,
	group_name TEXT NOT NULL DEFAULT '',
	allow      INTEGER NOT NULL,
	deny       INTEGER NOT NULL,
	PRIMARY KEY (channel_id, priority)
);
CREATE TABLE IF NOT EXISTS mumble_groups (
	channel_id  INTEGER NOT NULL,
	name        TEXT NOT NULL,
	inherit     INTEGER NOT NULL,
	inheritable INTEGER NOT NULL,
	PRIMARY KEY (channel_id, name)
);
CREATE TABLE IF NOT EXISTS mumble_group_members (
	channel_id INTEGER NOT NULL,
	group_name TEXT NOT NULL,
	user_id    INTEGER NOT NULL,
	removed    INTEGER NOT NULL, -- 0: added, 1: removed from inherited members
	PRIMARY KEY (channel_id, group_name, user_id)
);
CREATE TABLE IF NOT EXISTS mumble_bans (
	id       INTEGER PRIMARY KEY,
	prefix   TEXT NOT NULL DEFAULT '', -- "10.0.0.0/8", or '' for none
	hash     TEXT NOT NULL DEFAULT '',
	name     TEXT NOT NULL DEFAULT '',
	reason   TEXT NOT NULL DEFAULT '',
	start    INTEGER NOT NULL,
	duration INTEGER NOT NULL -- seconds, 0: forever
);
CREATE TABLE IF NOT EXISTS mumble_meta (
	key   TEXT PRIMARY KEY,
	value INTEGER NOT NULL
);
-- Root has a row too: its description, and links to it.
INSERT OR IGNORE INTO mumble_channels (id, parent, name) VALUES (0, 0, 'Root');
CREATE TABLE IF NOT EXISTS mumble_users (
	id            INTEGER PRIMARY KEY,
	name          TEXT NOT NULL,
	name_key      TEXT NOT NULL UNIQUE,
	cert_hash     TEXT UNIQUE,
	password_hash TEXT NOT NULL DEFAULT '',
	last_channel  INTEGER NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO mumble_users (id, name, name_key) VALUES (0, 'SuperUser', 'superuser');
CREATE TABLE IF NOT EXISTS mumble_channel_links (
	a INTEGER NOT NULL REFERENCES mumble_channels(id) ON DELETE CASCADE,
	b INTEGER NOT NULL REFERENCES mumble_channels(id) ON DELETE CASCADE,
	PRIMARY KEY (a, b)
);
`

// Columns added after the first version, for databases made before.
var migrations = []struct{ table, column, def string }{
	{"mumble_channels", "no_inherit_acl", "INTEGER NOT NULL DEFAULT 0"},
	{"mumble_users", "comment", "TEXT NOT NULL DEFAULT ''"},
	{"mumble_users", "texture", "BLOB"},
}

func OpenDB(db *sql.DB) (*DB, error) {
	if _, err := db.Exec(dbSchema); err != nil {
		return nil, err
	}
	for _, m := range migrations {
		if err := addColumn(db, m.table, m.column, m.def); err != nil {
			return nil, err
		}
	}
	d := &DB{db}
	return d, d.initACL()
}

func addColumn(db *sql.DB, table, column, def string) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + def)
	return err
}

// initACL gives a new database Murmur's default root ACL, once.
func (d *DB) initACL() error {
	var done int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM mumble_meta WHERE key = 'acl_initialized'`).Scan(&done)
	if err != nil || done > 0 {
		return err
	}
	root := &Channel{ID: 0, Name: "Root", Links: map[uint32]bool{}}
	root.ACLs, root.Groups = DefaultRootACL()
	if err := d.SaveChannel(root); err != nil {
		return err
	}
	_, err = d.db.Exec(`INSERT INTO mumble_meta (key, value) VALUES ('acl_initialized', 1)`)
	return err
}

// Channels returns the saved channels with their links.
func (d *DB) Channels() ([]Channel, error) {
	rows, err := d.db.Query(`SELECT id, parent, name, description, position, no_inherit_acl FROM mumble_channels`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[uint32]*Channel{}
	var order []uint32
	for rows.Next() {
		c := &Channel{Links: map[uint32]bool{}, Groups: map[string]*Group{}}
		if err := rows.Scan(&c.ID, &c.Parent, &c.Name, &c.Description, &c.Position, &c.NoInheritACL); err != nil {
			return nil, err
		}
		byID[c.ID] = c
		order = append(order, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	links, err := d.db.Query(`SELECT a, b FROM mumble_channel_links`)
	if err != nil {
		return nil, err
	}
	defer links.Close()
	for links.Next() {
		var a, b uint32
		if err := links.Scan(&a, &b); err != nil {
			return nil, err
		}
		if byID[a] != nil {
			byID[a].Links[b] = true
		}
	}
	if err := links.Err(); err != nil {
		return nil, err
	}
	if err := d.loadACLs(byID); err != nil {
		return nil, err
	}
	out := make([]Channel, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func (d *DB) loadACLs(byID map[uint32]*Channel) error {
	rows, err := d.db.Query(`SELECT channel_id, apply_here, apply_subs, user_id, group_name, allow, deny
		FROM mumble_acl ORDER BY channel_id, priority`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ch uint32
		var a ACL
		var user sql.NullInt64
		if err := rows.Scan(&ch, &a.ApplyHere, &a.ApplySubs, &user, &a.Group, &a.Allow, &a.Deny); err != nil {
			return err
		}
		a.HasUser, a.UserID = user.Valid, uint32(user.Int64)
		if c := byID[ch]; c != nil {
			c.ACLs = append(c.ACLs, a)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	groups, err := d.db.Query(`SELECT channel_id, name, inherit, inheritable FROM mumble_groups`)
	if err != nil {
		return err
	}
	defer groups.Close()
	for groups.Next() {
		var ch uint32
		g := newGroup("")
		if err := groups.Scan(&ch, &g.Name, &g.Inherit, &g.Inheritable); err != nil {
			return err
		}
		if c := byID[ch]; c != nil {
			c.Groups[g.Name] = g
		}
	}
	if err := groups.Err(); err != nil {
		return err
	}
	members, err := d.db.Query(`SELECT channel_id, group_name, user_id, removed FROM mumble_group_members`)
	if err != nil {
		return err
	}
	defer members.Close()
	for members.Next() {
		var ch, user uint32
		var name string
		var removed bool
		if err := members.Scan(&ch, &name, &user, &removed); err != nil {
			return err
		}
		if c := byID[ch]; c != nil && c.Groups[name] != nil {
			if removed {
				c.Groups[name].Remove[user] = true
			} else {
				c.Groups[name].Add[user] = true
			}
		}
	}
	return members.Err()
}

// SaveChannel writes a channel and its links (each link once, a < b).
func (d *DB) SaveChannel(c *Channel) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO mumble_channels (id, parent, name, description, position, no_inherit_acl) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET parent = excluded.parent, name = excluded.name,
		description = excluded.description, position = excluded.position, no_inherit_acl = excluded.no_inherit_acl`,
		c.ID, c.Parent, c.Name, c.Description, c.Position, c.NoInheritACL); err != nil {
		return err
	}
	if err := saveACLs(tx, c); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM mumble_channel_links WHERE a = ? OR b = ?`, c.ID, c.ID); err != nil {
		return err
	}
	for l := range c.Links {
		a, b := min(c.ID, l), max(c.ID, l)
		// The other channel may not be saved yet (it is saved next).
		if _, err := tx.Exec(`INSERT OR IGNORE INTO mumble_channel_links (a, b)
			SELECT ?, ? WHERE EXISTS (SELECT 1 FROM mumble_channels WHERE id = ?)`, a, b, l); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteChannels removes channels (a subtree) at once.
func (d *DB) DeleteChannels(ids []uint32) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM mumble_channel_links WHERE a = ? OR b = ?`, id, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM mumble_channels WHERE id = ? AND id != 0`, id); err != nil {
			return err
		}
		if id != 0 {
			if err := deleteACLs(tx, id); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// NextChannel is the highest channel id given out, so ids of removed and
// temporary channels are not reused.
// TextParent is the parent of the text channel mirrors (0: none yet).
func (d *DB) TextParent() (uint32, error) {
	var n uint32
	err := d.db.QueryRow(`SELECT value FROM mumble_meta WHERE key = 'text_parent'`).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}

func (d *DB) SetTextParent(id uint32) error {
	_, err := d.db.Exec(`INSERT INTO mumble_meta (key, value) VALUES ('text_parent', ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, id)
	return err
}

func (d *DB) NextChannel() (uint32, error) {
	var n uint32
	err := d.db.QueryRow(`SELECT value FROM mumble_meta WHERE key = 'last_channel_id'`).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}

func (d *DB) SaveNextChannel(n uint32) error {
	_, err := d.db.Exec(`INSERT INTO mumble_meta (key, value) VALUES ('last_channel_id', ?)
		ON CONFLICT (key) DO UPDATE SET value = max(value, excluded.value)`, n)
	return err
}

const userColumns = `id, name, COALESCE(cert_hash, ''), password_hash, last_channel`

func (d *DB) userWhere(where string, arg any) (*Registered, error) {
	var r Registered
	err := d.db.QueryRow(`SELECT `+userColumns+` FROM mumble_users WHERE `+where, arg).
		Scan(&r.ID, &r.Name, &r.CertHash, &r.PasswordHash, &r.LastChannel)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UserByName finds a registered user by name (compared as names.Key).
func (d *DB) UserByName(name string) (*Registered, error) {
	return d.userWhere(`name_key = ?`, names.Key(name))
}

// CheckPassword logs in a registration that has a password (SuperUser,
// for example) and returns its name.
func (d *DB) CheckPassword(name, pw string) (string, bool) {
	r, err := d.UserByName(name)
	if err != nil || r == nil || r.PasswordHash == "" {
		return "", false
	}
	return r.Name, checkPassword(r.PasswordHash, pw)
}

func (d *DB) UserByHash(hash string) (*Registered, error) {
	if hash == "" {
		return nil, nil
	}
	return d.userWhere(`cert_hash = ?`, hash)
}

// RegisterUser adds a user with a certificate and returns its id.
func (d *DB) RegisterUser(name, certHash string) (uint32, error) {
	res, err := d.db.Exec(`INSERT INTO mumble_users (name, name_key, cert_hash) VALUES (?, ?, NULLIF(?, ''))`,
		name, names.Key(name), certHash)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint32(id), err
}

func (d *DB) SetPassword(id uint32, passwordHash string) error {
	_, err := d.db.Exec(`UPDATE mumble_users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	return err
}

func (d *DB) SetLastChannel(id, channel uint32) error {
	_, err := d.db.Exec(`UPDATE mumble_users SET last_channel = ? WHERE id = ?`, channel, id)
	return err
}

// UserNames returns the names of the registered users among ids.
func (d *DB) UserNames(ids []uint32) (map[uint32]string, error) {
	out := map[uint32]string{}
	for _, id := range ids {
		var name string
		err := d.db.QueryRow(`SELECT name FROM mumble_users WHERE id = ?`, id).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, nil
}

func deleteACLs(tx *sql.Tx, channel uint32) error {
	for _, table := range []string{"mumble_acl", "mumble_groups", "mumble_group_members"} {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE channel_id = ?`, channel); err != nil {
			return err
		}
	}
	return nil
}

// saveACLs replaces a channel's ACL entries and groups.
func saveACLs(tx *sql.Tx, c *Channel) error {
	if err := deleteACLs(tx, c.ID); err != nil {
		return err
	}
	for i, a := range c.ACLs {
		var user any
		if a.HasUser {
			user = a.UserID
		}
		if _, err := tx.Exec(`INSERT INTO mumble_acl (channel_id, priority, apply_here, apply_subs, user_id, group_name, allow, deny)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, c.ID, i, a.ApplyHere, a.ApplySubs, user, a.Group, a.Allow, a.Deny); err != nil {
			return err
		}
	}
	for _, g := range c.Groups {
		if _, err := tx.Exec(`INSERT INTO mumble_groups (channel_id, name, inherit, inheritable) VALUES (?, ?, ?, ?)`,
			c.ID, g.Name, g.Inherit, g.Inheritable); err != nil {
			return err
		}
		for removed, ids := range []map[uint32]bool{g.Add, g.Remove} {
			for id := range ids {
				if _, err := tx.Exec(`INSERT OR REPLACE INTO mumble_group_members (channel_id, group_name, user_id, removed)
					VALUES (?, ?, ?, ?)`, c.ID, g.Name, id, removed); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Users lists the registered users without SuperUser.
func (d *DB) Users() ([]Registered, error) {
	rows, err := d.db.Query(`SELECT ` + userColumns + ` FROM mumble_users WHERE id != 0 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Registered
	for rows.Next() {
		var r Registered
		if err := rows.Scan(&r.ID, &r.Name, &r.CertHash, &r.PasswordHash, &r.LastChannel); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *DB) RenameUser(id uint32, name string) error {
	if why := names.Check(name); why != "" || id == 0 {
		return errors.New("invalid name")
	}
	_, err := d.db.Exec(`UPDATE mumble_users SET name = ?, name_key = ? WHERE id = ?`, name, names.Key(name), id)
	return err
}

// UnregisterUser removes a user and its ACL entries and group memberships.
func (d *DB) UnregisterUser(id uint32) error {
	if id == 0 {
		return errors.New("SuperUser cannot be removed")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM mumble_users WHERE id = ?`,
		`DELETE FROM mumble_acl WHERE user_id = ?`,
		`DELETE FROM mumble_group_members WHERE user_id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) Bans() ([]BanEntry, error) {
	rows, err := d.db.Query(`SELECT prefix, hash, name, reason, start, duration FROM mumble_bans ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BanEntry
	for rows.Next() {
		var b BanEntry
		var prefix string
		var start, duration int64
		if err := rows.Scan(&prefix, &b.Hash, &b.Name, &b.Reason, &start, &duration); err != nil {
			return nil, err
		}
		if prefix != "" {
			if b.Prefix, err = netip.ParsePrefix(prefix); err != nil {
				return nil, err
			}
		}
		b.Start, b.Duration = time.Unix(start, 0).UTC(), time.Duration(duration)*time.Second
		out = append(out, b)
	}
	return out, rows.Err()
}

// SaveBans replaces the ban list.
func (d *DB) SaveBans(bans []BanEntry) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM mumble_bans`); err != nil {
		return err
	}
	for _, b := range bans {
		prefix := ""
		if b.Prefix.IsValid() {
			prefix = b.Prefix.String()
		}
		if _, err := tx.Exec(`INSERT INTO mumble_bans (prefix, hash, name, reason, start, duration) VALUES (?, ?, ?, ?, ?, ?)`,
			prefix, b.Hash, b.Name, b.Reason, b.Start.Unix(), int64(b.Duration/time.Second)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UserContent is a registered user's comment and avatar (read at login
// only: avatars are large).
func (d *DB) UserContent(id uint32) (comment string, texture []byte, err error) {
	err = d.db.QueryRow(`SELECT comment, texture FROM mumble_users WHERE id = ?`, id).Scan(&comment, &texture)
	return comment, texture, err
}

// textureByName is a registered user's avatar (nil: none, or no such user).
func (d *DB) textureByName(name string) ([]byte, error) {
	var texture []byte
	err := d.db.QueryRow(`SELECT texture FROM mumble_users WHERE name_key = ?`, names.Key(name)).Scan(&texture)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return texture, err
}

func (d *DB) SetUserContent(id uint32, comment string, texture []byte) error {
	_, err := d.db.Exec(`UPDATE mumble_users SET comment = ?, texture = ? WHERE id = ?`, comment, texture, id)
	return err
}

// InitLobby makes root a lobby without voice, as Kallisti servers have it:
// users arrive there and join voice channels on purpose. A new server gets
// a "General" voice channel. Runs once per database.
// InitRoles gives an older database the moderator group and its ACL in
// root, once.
func (d *DB) InitRoles() error {
	var done int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM mumble_meta WHERE key = 'roles'`).Scan(&done); err != nil || done > 0 {
		return err
	}
	saved, err := d.Channels()
	if err != nil {
		return err
	}
	root := NewState("Root", saved).Channels[0]
	if root.Groups["moderator"] == nil {
		if root.Groups == nil {
			root.Groups = map[string]*Group{}
		}
		root.Groups["moderator"] = newGroup("moderator")
		root.ACLs = append(root.ACLs, moderatorACL()...)
		if err := d.SaveChannel(root); err != nil {
			return err
		}
	}
	_, err = d.db.Exec(`INSERT INTO mumble_meta (key, value) VALUES ('roles', 1)`)
	return err
}

func (d *DB) InitLobby() error {
	var done int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM mumble_meta WHERE key = 'lobby'`).Scan(&done); err != nil || done > 0 {
		return err
	}
	saved, err := d.Channels()
	if err != nil {
		return err
	}
	s := NewState("Root", saved)
	root := s.Channels[0]
	root.ACLs = append(root.ACLs, ACL{ApplyHere: true, ApplySubs: false, Group: "all", Deny: Speak})
	if err := d.SaveChannel(root); err != nil {
		return err
	}
	if len(s.Channels) == 1 {
		last, err := d.NextChannel()
		if err != nil {
			return err
		}
		s.SetLastChannelID(last)
		general, err := s.AddChannel(0, "General", false)
		if err != nil {
			return err
		}
		if err := d.SaveChannel(general); err != nil {
			return err
		}
		if err := d.SaveNextChannel(general.ID); err != nil {
			return err
		}
	}
	_, err = d.db.Exec(`INSERT INTO mumble_meta (key, value) VALUES ('lobby', 1)`)
	return err
}
