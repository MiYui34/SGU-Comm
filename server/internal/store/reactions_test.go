package store

import (
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestReactions(t *testing.T) {
	s := open(t)
	if err := s.EnsureChannels([]string{"general"}); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	m, _ := s.AddMessage(NewMessage{ChannelID: 1, Author: "alice", Text: "hi", At: at})
	other, _ := s.AddMessage(NewMessage{ChannelID: 1, Author: "bob", Text: "yo", At: at})
	bob, carol := uint32(7), uint32(8)
	for _, r := range []struct {
		emoji string
		id    uint32
		name  string
		on    bool
	}{{"👍", bob, "bob", true}, {"👍", carol, "carol", true}, {"🎉", bob, "bob", true}, {"🎉", bob, "bob", false}} {
		if _, err := s.SetReaction(m.ID, r.emoji, r.id, r.name, r.on, at); err != nil {
			t.Fatal(err)
		}
	}
	// The user id counts, not the name: renamed, still the same user.
	if changed, _ := s.SetReaction(m.ID, "👍", bob, "bobby", true, at); changed {
		t.Fatal("adding again is no change")
	}
	if changed, _ := s.SetReaction(m.ID, "🎉", bob, "bob", false, at); changed {
		t.Fatal("removing again is no change")
	}
	want := []Reaction{{Emoji: "👍", Users: []string{"bob", "carol"}, IDs: []uint32{bob, carol}}}
	if got, _ := s.MessageByID(m.ID); !reflect.DeepEqual(got.Reactions, want) {
		t.Fatalf("message: %+v", got.Reactions)
	}
	page, _ := s.History(1, 0, 10)
	if !reflect.DeepEqual(page[0].Reactions, want) || page[1].Reactions != nil {
		t.Fatalf("history: %+v / %+v", page[0].Reactions, page[1].Reactions)
	}
	// Not on a message that does not exist.
	if _, err := s.SetReaction(9999, "👍", 7, "bob", true, at); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing message: %v", err)
	}
	// A limit on different emoji per message.
	for i := range maxReactionKinds {
		if _, err := s.SetReaction(other.ID, "e"+strconv.Itoa(i), 7, "bob", true, at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.SetReaction(other.ID, "one more", 7, "bob", true, at); !errors.Is(err, ErrTooManyReactions) {
		t.Fatalf("over the limit: %v", err)
	}
	if _, err := s.SetReaction(other.ID, "e0", 8, "carol", true, at); err != nil {
		t.Fatalf("a known emoji is fine: %v", err)
	}
	// Gone with the message.
	if _, err := s.DeleteMessage(m.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT count(*) FROM reactions WHERE message_id = ?`, m.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d reactions left", n)
	}
}

// Reactions of the first version were kept by name: those of registered
// users (built-in server, same database) get their ids, the rest go.
func TestReactionsByNameAreMigrated(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	at := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	m, _ := s.AddMessage(NewMessage{ChannelID: 1, Author: "alice", Text: "hi", At: at})
	for _, q := range []string{
		`DROP TABLE reactions`,
		`CREATE TABLE reactions (message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE, emoji TEXT NOT NULL,
			user_key TEXT NOT NULL, user TEXT NOT NULL, created_at INTEGER NOT NULL, PRIMARY KEY (message_id, emoji, user_key))`,
		`CREATE TABLE mumble_users (id INTEGER PRIMARY KEY, name TEXT, name_key TEXT)`,
		`INSERT INTO mumble_users VALUES (5, 'Bob', 'bob')`,
		`INSERT INTO reactions VALUES (1, '👍', 'bob', 'Bob', 1), (1, '👍', 'guest', 'guest', 2)`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	if err := migrate(s.db); err != nil {
		t.Fatal(err)
	}
	got, _ := s.MessageByID(m.ID)
	if want := []Reaction{{Emoji: "👍", Users: []string{"Bob"}, IDs: []uint32{5}}}; !reflect.DeepEqual(got.Reactions, want) {
		t.Fatalf("%+v", got.Reactions)
	}
}
