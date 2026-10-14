package mumbled

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1) // one in-memory database
	t.Cleanup(func() { sqlDB.Close() })
	db, err := OpenDB(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestChannelsAreSavedWithLinks(t *testing.T) {
	db := testDB(t)
	s := NewState("Root", nil)
	a, _ := s.AddChannel(0, "A", false)
	b, _ := s.AddChannel(a.ID, "B", false)
	b.Description, b.Position = "about B", 3
	s.Link(a.ID, b.ID)
	for _, c := range []*Channel{a, b} {
		if err := db.SaveChannel(c); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := db.Channels()
	if err != nil {
		t.Fatal(err)
	}
	loaded := NewState("Root", saved)
	got := loaded.Channels[b.ID]
	if got == nil || got.Parent != a.ID || got.Description != "about B" || got.Position != 3 || !got.Links[a.ID] {
		t.Fatalf("loaded %+v", got)
	}
	// New channels do not reuse ids.
	c, _ := loaded.AddChannel(0, "C", false)
	if c.ID <= b.ID {
		t.Fatalf("id %d reused", c.ID)
	}
	if err := db.DeleteChannels([]uint32{a.ID}); err != nil {
		t.Fatal(err)
	}
	saved, _ = db.Channels()
	if len(saved) != 2 || len(NewState("Root", saved).Channels[b.ID].Links) != 0 {
		t.Fatalf("after delete: %+v", saved)
	}
}

func TestLinksToRootAndIDsSurviveRestarts(t *testing.T) {
	db := testDB(t)
	saved, _ := db.Channels()
	s := NewState("Root", saved)
	s.nextChannel = db.mustNext(t)
	a, _ := s.AddChannel(0, "A", false)
	b, _ := s.AddChannel(0, "B", false)
	s.Link(a.ID, 0)
	db.SaveChannel(a)
	db.SaveChannel(b)
	db.SaveNextChannel(s.nextChannel)
	if err := db.DeleteChannels([]uint32{b.ID}); err != nil {
		t.Fatal(err)
	}
	saved, _ = db.Channels()
	s2 := NewState("Root", saved)
	s2.nextChannel = db.mustNext(t)
	if !s2.Channels[a.ID].Links[0] {
		t.Fatal("link to root lost")
	}
	c, _ := s2.AddChannel(0, "C", false)
	if c.ID <= b.ID {
		t.Fatalf("id %d of a deleted channel reused", c.ID)
	}
}

func (d *DB) mustNext(t *testing.T) uint32 {
	t.Helper()
	n, err := d.NextChannel()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRegisteredUsers(t *testing.T) {
	db := testDB(t)
	id, err := db.RegisterUser("Alice", "aaa")
	if err != nil || id == 0 {
		t.Fatalf("id %d err %v", id, err)
	}
	if _, err := db.RegisterUser("alice", "bbb"); err == nil {
		t.Fatal("registered a taken name")
	}
	byName, _ := db.UserByName("ALICE")
	byHash, _ := db.UserByHash("aaa")
	if byName == nil || byHash == nil || byName.ID != id || byHash.Name != "Alice" {
		t.Fatalf("by name %+v by hash %+v", byName, byHash)
	}
	if u, _ := db.UserByHash(""); u != nil {
		t.Fatal("found a user by an empty hash")
	}
	db.SetLastChannel(id, 7)
	byName, _ = db.UserByName("alice")
	if byName.LastChannel != 7 {
		t.Fatalf("last channel %d", byName.LastChannel)
	}
	// SuperUser exists from the start, with a password set by the admin.
	if err := db.SetPassword(0, "hash:pw"); err != nil {
		t.Fatal(err)
	}
	super, _ := db.UserByName("superuser")
	if super == nil || super.ID != 0 || super.PasswordHash != "hash:pw" {
		t.Fatalf("superuser %+v", super)
	}
	got, _ := db.UserNames([]uint32{0, id, 99})
	if got[id] != "Alice" || got[0] != "SuperUser" || len(got) != 2 {
		t.Fatalf("names %v", got)
	}
}

func TestNewDatabasesHaveTheDefaultACL(t *testing.T) {
	db := testDB(t)
	saved, _ := db.Channels()
	s := NewState("Root", saved)
	acls, groups := DefaultRootACL()
	if len(s.Channels[0].ACLs) != len(acls) || s.Channels[0].Groups["admin"] == nil || len(groups) != 1 {
		t.Fatalf("root ACL %+v groups %+v", s.Channels[0].ACLs, s.Channels[0].Groups)
	}
	// An admin who removes all root ACLs keeps it that way after a restart.
	s.Channels[0].ACLs = nil
	db.SaveChannel(s.Channels[0])
	db2, err := OpenDB(db.db)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ = db2.Channels()
	if len(NewState("Root", saved).Channels[0].ACLs) != 0 {
		t.Fatal("default ACL came back")
	}
}

func TestACLsAndGroupsAreSaved(t *testing.T) {
	db := testDB(t)
	s := NewState("Root", nil)
	a, _ := s.AddChannel(0, "A", false)
	a.NoInheritACL = true
	a.ACLs = []ACL{
		{ApplyHere: true, ApplySubs: false, Group: "all", Deny: Enter},
		{ApplyHere: true, ApplySubs: true, HasUser: true, UserID: 5, Allow: Enter | Speak},
	}
	g := newGroup("mods")
	g.Inheritable = false
	g.Add[5], g.Remove[6] = true, true
	a.Groups = map[string]*Group{"mods": g}
	if err := db.SaveChannel(a); err != nil {
		t.Fatal(err)
	}
	saved, _ := db.Channels()
	got := NewState("Root", saved).Channels[a.ID]
	if !got.NoInheritACL || len(got.ACLs) != 2 || got.ACLs[0] != a.ACLs[0] || got.ACLs[1] != a.ACLs[1] {
		t.Fatalf("ACLs %+v", got.ACLs)
	}
	mods := got.Groups["mods"]
	if mods == nil || mods.Inheritable || !mods.Inherit || !mods.Add[5] || !mods.Remove[6] {
		t.Fatalf("group %+v", mods)
	}
	db.DeleteChannels([]uint32{a.ID})
	var n int
	db.db.QueryRow(`SELECT (SELECT COUNT(*) FROM mumble_acl WHERE channel_id = ?) + (SELECT COUNT(*) FROM mumble_groups WHERE channel_id = ?)`, a.ID, a.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d ACL rows left", n)
	}
}

func TestOldDatabasesGetTheNewColumn(t *testing.T) {
	sqlDB, _ := sql.Open("sqlite", ":memory:")
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	sqlDB.Exec(`CREATE TABLE mumble_channels (id INTEGER PRIMARY KEY, parent INTEGER NOT NULL, name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '', position INTEGER NOT NULL DEFAULT 0)`)
	if _, err := OpenDB(sqlDB); err != nil {
		t.Fatal(err)
	}
}
