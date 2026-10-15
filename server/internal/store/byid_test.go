package store

import (
	"testing"
	"time"
)

func TestAuthorsAndReadMarksByUserID(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	at := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	m, _ := s.AddMessage(NewMessage{ChannelID: 1, Author: "alice", Verified: true, AuthorID: 4, Text: "hi", At: at})
	g, _ := s.AddMessage(NewMessage{ChannelID: 1, Author: "guest", AuthorID: 9, Text: "yo", At: at})
	if got, _ := s.MessageByID(m.ID); got.AuthorID == nil || *got.AuthorID != 4 {
		t.Fatalf("verified %+v", got.AuthorID)
	}
	if got, _ := s.MessageByID(g.ID); got.AuthorID != nil {
		t.Fatalf("guest %+v", got.AuthorID)
	}
	// SuperUser is 0, a real id.
	su, _ := s.AddMessage(NewMessage{ChannelID: 1, Author: "SuperUser", Verified: true, AuthorID: 0, Text: "x", At: at})
	if got, _ := s.MessageByID(su.ID); got.AuthorID == nil || *got.AuthorID != 0 {
		t.Fatalf("SuperUser %+v", got.AuthorID)
	}
	if err := s.SetReadMark(4, 1, m.ID); err != nil {
		t.Fatal(err)
	}
	if marks, _ := s.ReadMarks(4); marks[1] != m.ID {
		t.Fatalf("marks %v", marks)
	}
}

// Databases from before: verified authors and read marks by name get the
// ids of the built-in server's registrations (same database).
func TestAuthorsAndReadMarksByNameAreMigrated(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	at := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	m, _ := s.AddMessage(NewMessage{ChannelID: 1, Author: "Ünal", Verified: true, AuthorID: 5, Text: "hi", At: at})
	other, _ := s.AddMessage(NewMessage{ChannelID: 1, Author: "nobody", Verified: true, AuthorID: 6, Text: "x", At: at})
	for _, q := range []string{
		`UPDATE messages SET author_id = NULL`,
		`CREATE TABLE IF NOT EXISTS read_marks (user TEXT NOT NULL, channel_id INTEGER NOT NULL, message_id INTEGER NOT NULL, PRIMARY KEY (user, channel_id))`,
		`INSERT INTO read_marks VALUES ('ünal', 1, 1), ('gone', 1, 1)`,
		`CREATE TABLE mumble_users (id INTEGER PRIMARY KEY, name TEXT, name_key TEXT)`,
		`INSERT INTO mumble_users VALUES (5, 'Ünal', 'ünal')`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	if err := migrate(s.db); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.MessageByID(m.ID); got.AuthorID == nil || *got.AuthorID != 5 {
		t.Fatalf("author %+v", got.AuthorID)
	}
	if got, _ := s.MessageByID(other.ID); got.AuthorID != nil || got.Verified {
		t.Fatalf("unknown author: id %+v, verified %v", got.AuthorID, got.Verified)
	}
	if marks, _ := s.ReadMarks(5); marks[1] != 1 {
		t.Fatalf("marks %v", marks)
	}
}
