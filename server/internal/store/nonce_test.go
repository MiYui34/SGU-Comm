package store

import (
	"errors"
	"testing"
	"time"
)

func TestMessageByNonce(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	alice := NewMessage{ChannelID: 1, Author: "alice", Verified: true, AuthorID: 3, Text: "hi", At: at, Nonce: "n1"}
	m, err := s.AddMessage(alice)
	if err != nil {
		t.Fatal(err)
	}
	// The same author (by user id: also after a rename) finds it within a day.
	retry := alice
	retry.Author, retry.At = "alice2", at.Add(time.Hour)
	if got, err := s.MessageByNonce(retry); err != nil || got.ID != m.ID {
		t.Fatalf("retry: %+v %v", got, err)
	}
	// Another author with the same nonce, a guest with the same name, or
	// no nonce: not found.
	for _, n := range []NewMessage{
		{Author: "bob", Verified: true, AuthorID: 4, Nonce: "n1", At: at},
		{Author: "alice", Nonce: "n1", At: at},
		{Author: "alice", Verified: true, AuthorID: 3, At: at},
	} {
		if _, err := s.MessageByNonce(n); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%+v: %v", n, err)
		}
	}
	// A day later the nonce is forgotten: the same one makes a new message.
	later := alice
	later.At = at.Add(25 * time.Hour)
	if _, err := s.MessageByNonce(later); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired: %v", err)
	}
	again, err := s.AddMessage(later)
	if err != nil || again.ID == m.ID {
		t.Fatalf("after a day: %+v %v", again, err)
	}
	if got, _ := s.MessageByNonce(later); got.ID != again.ID {
		t.Fatalf("newest: %+v", got)
	}
	// Guests by name.
	guest := NewMessage{ChannelID: 1, Author: "carol", Text: "yo", At: at, Nonce: "n2"}
	g, _ := s.AddMessage(guest)
	if got, err := s.MessageByNonce(guest); err != nil || got.ID != g.ID {
		t.Fatalf("guest: %+v %v", got, err)
	}
}

func TestNoncesAreUniquePerAuthor(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	at := time.Now()
	n := NewMessage{ChannelID: 1, Author: "alice", Text: "hi", At: at, Nonce: "n1"}
	if _, err := s.AddMessage(n); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddMessage(n); err == nil {
		t.Fatal("a second message with the same nonce")
	}
	// Without a nonce, as before: any number.
	n.Nonce = ""
	for range 2 {
		if _, err := s.AddMessage(n); err != nil {
			t.Fatal(err)
		}
	}
}
