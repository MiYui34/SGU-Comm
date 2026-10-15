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
	for _, r := range []struct {
		emoji, user string
		on          bool
	}{{"👍", "bob", true}, {"👍", "carol", true}, {"🎉", "bob", true}, {"👍", "Bob", true}, {"🎉", "bob", false}} {
		if err := s.SetReaction(m.ID, r.emoji, r.user, r.on, at); err != nil {
			t.Fatal(err)
		}
	}
	want := []Reaction{{Emoji: "👍", Users: []string{"bob", "carol"}}} // "Bob" is bob
	if got, _ := s.MessageByID(m.ID); !reflect.DeepEqual(got.Reactions, want) {
		t.Fatalf("message: %+v", got.Reactions)
	}
	page, _ := s.History(1, 0, 10)
	if !reflect.DeepEqual(page[0].Reactions, want) || page[1].Reactions != nil {
		t.Fatalf("history: %+v / %+v", page[0].Reactions, page[1].Reactions)
	}
	// Not on a message that does not exist.
	if err := s.SetReaction(9999, "👍", "bob", true, at); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing message: %v", err)
	}
	// A limit on different emoji per message.
	for i := range maxReactionKinds {
		if err := s.SetReaction(other.ID, "e"+strconv.Itoa(i), "bob", true, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetReaction(other.ID, "one more", "bob", true, at); !errors.Is(err, ErrTooManyReactions) {
		t.Fatalf("over the limit: %v", err)
	}
	if err := s.SetReaction(other.ID, "e0", "carol", true, at); err != nil {
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
